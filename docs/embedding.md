# Embedding tix

tix can run inside another program's process instead of as `tix serve`. The program gets a URL on
`127.0.0.1` and talks to it with the ordinary [HTTP API](api.md), over a SQLite file it owns. Nothing else
is installed and no other process starts.

This exists for [tix-mobile](https://github.com/heliopsy/tix-mobile), which works on a local `.db` file on
Android, iOS, Linux, macOS and Windows. iOS and Android do not let an app start a second process, so the
`tix` binary cannot be shipped beside the app there; the app loads tix as a library instead.

What runs is exactly what `tix serve --db FILE` runs on a machine with nothing configured: the API, the
browser interface, the lease sweeper, the snooze waker, the retention pruner, the webhook dispatcher and
the server registration. Both build their server through one function, `internal/serving.Build`, so they
cannot drift apart. The SSH listener is the one part left out.

## From Go

```go
import "github.com/heliopsy/tix/embedded"

in, err := embedded.Start(ctx, embedded.Options{DBPath: path, InitIfMissing: true})
if err != nil { ... }
defer in.Stop(context.Background())

token, err := in.Token(ctx, embedded.TokenRequest{Name: "app"})
// GET in.URL() + "/api/v1/whoami" with "Authorization: Bearer " + token
```

| Call | Does |
| --- | --- |
| `Start(ctx, Options)` | Opens or creates the database, starts the API and the workers, returns once the URL accepts connections |
| `URL()` | The base URL, `http://127.0.0.1:PORT` |
| `Stop(ctx)` | Drains in-flight requests, stops the workers, closes the database; a second call does nothing |
| `Tenants(ctx)` | Every live tenant, by key and name |
| `CreateTenant(ctx, key, name)` | A tenant with no members, as `tix tenant create` makes one |
| `Token(ctx, TokenRequest)` | An API token for the local owner of a tenant, as `tix token create --db` mints one |
| `ErrorCode(err)` | The error's kind: `invalid`, `not_found`, `conflict`, `forbidden`, `internal` and the rest |

`Options.InitIfMissing` creates a missing file and seeds it as `tix init` does, with the default tenant and
its starter projects. Without it a missing file is a `not_found` error and nothing is created, so a typo in a
path cannot quietly start an empty database.

A token request with no scopes gets every scope (`*`). Its lifetime is 90 days unless `TTL` says otherwise;
a negative `TTL` asks for a token that never expires. The token belongs to the tenant's local actor, the
identity a shell holding the database acts as, so the app never needs a password for its own file.

The package reads no configuration: no `TIX_*` variables, no configuration file, no `.env`. An app's
environment is not a tix operator's, and a stray `~/.config/tix` changing what the app's database does would
be impossible to reproduce. Everything is at its default.

## The C ABI

`embedded/libtix` exports the package to C, for `dart:ffi` and anything else that loads a shared library.
Arguments and answers are JSON strings; an instance is addressed by the integer handle `TixStart` returns.

| Function | Takes | Answers |
| --- | --- | --- |
| `char* TixStart(const char* options)` | `{"db_path", "listen"?, "init_if_missing"?}` | `{"handle":1,"url":"http://127.0.0.1:PORT"}` |
| `char* TixStop(long long handle)` | a handle | `{}` |
| `char* TixTenants(long long handle)` | a handle | `{"tenants":[{"key","name"}]}` |
| `char* TixCreateTenant(long long handle, const char* json)` | `{"key","name"}` | `{"key","name"}` |
| `char* TixToken(long long handle, const char* json)` | `{"tenant","name","scopes"?,"ttl"?}` | `{"token":"tix_pat_..."}` |
| `char* TixVersion(void)` | nothing | `{"version","commit"}` |
| `void TixFree(char* p)` | a string any function above returned | nothing |

Any function but `TixFree` may instead answer `{"error":{"code":"...","message":"..."}}`. The code is the
tix error kind, as the HTTP API reports it. Malformed JSON and an unknown field are `invalid`, an unknown or
stopped handle `not_found`, and a panic inside tix `internal`, returned like any other error rather than
taking the calling process down.

`ttl` is a Go duration (`720h`), a number of days (`90d`) or `never`; omitted, it is 90 days. `tenant`
omitted is the default tenant.

Every returned string is allocated with `malloc` and must be released with `TixFree`, never with the
caller's own allocator. The generated header declares the string parameters as `char*` rather than
`const char*`, because cgo cannot emit `const`; the library never writes through them.

## Threads and lifecycle

- Every function is safe to call from any thread at the same time. The handle table is behind a lock, and
  each instance's own work is the server's, which is concurrent already.
- The calls block. `TixStart` opens and migrates the database, which on a large or old file takes a moment,
  and `TixStop` waits for in-flight requests (up to 15 seconds, then it closes them). From Dart, call them
  from a background isolate, not the UI isolate.
- Several instances run at once, one per database file, each on its own port. Two instances on the same file
  are possible, as two `tix` processes are, but pointless.
- A handle stops being valid when `TixStop` returns; calling with it again answers `not_found`.
- The workers wake on their intervals: the lease sweep every minute, the server heartbeat every 30 seconds,
  the webhook dispatcher, hourly pruning. Stop the instance when the app goes to the background. iOS suspends
  it anyway, and the port will not answer until the app is in the foreground again.
- Two instances in one process share one process-wide connection counter, so `tix status` against either
  file may report the other's live connection count. The data and the serving are separate.

## Security

The listener is on loopback only. `listen` must be a loopback IP literal (`127.0.0.1:0` by default, or
`[::1]:0`); a hostname, `localhost` included, or any other address is refused before the database is
opened. Nothing on the network can reach the instance.

Loopback is not private on a multi-user desktop: another local user could connect to the port. Every API
route still needs a token or a session, exactly as under `tix serve`, so the app should keep the token it
mints to itself (the platform keychain, not a world-readable file) and mint one per install rather than
reusing one across machines.

The token is issued to the database's local actor, which a shell holding the file already acts as. Owning
the file is owning the data; the library adds no new way in.

## Building

The library is the only cgo in the repository. `CGO_ENABLED=0 go build ./...` skips it, nothing the `tix`
binary is built from imports it, and `TestTheBinaryReachesNoCgo` in `internal/architecture` fails if that
ever changes. Each recipe writes the library, its header `libtix.h` and `NOTICES.json` to
`out/lib/<platform>/`.

| Recipe | Output | Needs |
| --- | --- | --- |
| `just lib` | `out/lib/<os>-<arch>/libtix.so`, `.dylib` or `.dll` for this machine | a C compiler (gcc, clang, or mingw on Windows) |
| `just lib-android ABI` | `out/lib/android/<ABI>/libtix.so` for `arm64-v8a`, `armeabi-v7a`, `x86_64` or `x86` | the Android NDK, r23 or newer, in `ANDROID_NDK_HOME` |
| `just lib-android-all` | all four ABIs | the NDK |
| `just lib-macos` | `out/lib/macos/libtix.dylib`, universal arm64 and x86_64, install name `@rpath/libtix.dylib` | Xcode |
| `just lib-ios` | `out/lib/ios/Tix.xcframework`: static `libtix.a` for devices (arm64) and the simulator (arm64, x86_64) | Xcode |
| `just lib-windows ARCH` | `out/lib/windows-<arch>/libtix.dll` | mingw-w64 (`x86_64-w64-mingw32-gcc`), or `CC=gcc` on Windows |
| `just lib-notices GOOS GOARCH DIR` | `DIR/NOTICES.json` | Go |
| `just lib-test` | the package and C ABI tests, including a C program driving the built library | a C compiler |

`ANDROID_API` sets the minimum Android API level, 21 by default. iOS targets 13.0, macOS 11.0.

The library carries the whole server, the browser assets included: about 32 MB per Android ABI and 34 MB on
Linux amd64, stripped, before the store compresses it.

### Android and 16 KB pages

Devices running Android 15 and later may use 16 KB memory pages, and Google Play requires every native
library in an app targeting them to be 16 KB aligned. `just lib-android` links with
`-Wl,-z,max-page-size=16384`, which NDK r28 and later do by default and older NDKs need, and then runs
`just lib-android-check`, which fails unless every `LOAD` segment `llvm-readelf -l` lists is aligned to
`0x4000`. CI builds all four ABIs and runs that check on every change.

### Licence notices

The library links its Go dependencies statically, so an app shipping it ships their licences too.
`just lib-notices` (run by every build recipe) lists the modules the library links for that target with
`go list -deps`, reads each one's licence file from the module cache, and writes:

```json
{
  "package": "./embedded/libtix",
  "target": "android/arm64",
  "modules": [
    {
      "module": "modernc.org/sqlite",
      "version": "v1.61.1",
      "license": "BSD-3-Clause",
      "license_text": "Copyright (c) 2017 The Sqlite Authors. ...",
      "other_files": [{"name": "LICENSE-SQLITE", "text": "..."}]
    }
  ]
}
```

`license` is an SPDX identifier. `other_files` holds a module's further licence and notice files, such as an
Apache `NOTICE` or a third-party licence list. The Go standard library is the module `std`. tix's own entry
is `AGPL-3.0-or-later` and carries `REUSE.toml` and `LICENSES/`, which name the scripts vendored into the
browser interface and their licences. A licence the generator cannot identify fails the recipe rather than
being written down as unknown; teach `internal/notices` its fingerprint.

## Loading it from an app

| Platform | File | How it is loaded |
| --- | --- | --- |
| Android | `libtix.so` per ABI | Put each in `android/app/src/main/jniLibs/<ABI>/` (`arm64-v8a`, `armeabi-v7a`, `x86_64`, `x86`); `DynamicLibrary.open('libtix.so')` |
| iOS | `Tix.xcframework` | Link it statically into the Runner target; `DynamicLibrary.process()` |
| macOS | `libtix.dylib` | Embed it in `Contents/Frameworks`; `DynamicLibrary.open('libtix.dylib')` (install name `@rpath/libtix.dylib`) |
| Linux | `libtix.so` | Install it beside the executable under `lib/`; `DynamicLibrary.open('libtix.so')` |
| Windows | `libtix.dll` | Beside the executable; `DynamicLibrary.open('libtix.dll')` |

On iOS the archive is linked into the app binary, so the symbols are looked up in the process. The linker
drops symbols nothing references, so keep them: either reference each `Tix*` function from Objective-C or
Swift, or link with `-force_load` on the archive (Other Linker Flags) so the dead-strip pass leaves them. The
Go runtime in the archive also needs `-lresolv` and the `CoreFoundation` and `Security` frameworks.

On Android, 16 KB alignment also applies to how the library sits in the APK when it is stored uncompressed:
build with Android Gradle Plugin 8.5.1 or newer, which aligns it, and leave `useLegacyPackaging` off.

## Not in releases yet

The libraries are built and tested in CI and uploaded as workflow artifacts for a week, not attached to
releases. Release wiring would add a matrix to the release workflow (Linux for Linux, Android and Windows;
macOS for macOS and iOS), tar each platform with its header and `NOTICES.json`, add the archives to
`checksums.txt` and sign them with the same keyless cosign step the binaries get.
