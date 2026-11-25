// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Drives libtix through its C ABI exactly as a foreign caller would. Every
// answer is printed on its own line, prefixed by a label, for abi_test.go to
// check. Between minting a token and stopping it waits for a line on stdin,
// so the test can call the running instance over HTTP first.

#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "libtix.h"

static void show(const char *label, char *json) {
    printf("%s %s\n", label, json);
    fflush(stdout);
    TixFree(json);
}

static long long handle_of(const char *json) {
    const char *at = strstr(json, "\"handle\":");
    return at ? atoll(at + strlen("\"handle\":")) : -1;
}

static long long shared;
static int bad_answers;

static void *list_tenants(void *arg) {
    (void)arg;
    for (int i = 0; i < 20; i++) {
        char *json = TixTenants(shared);
        if (strstr(json, "{\"tenants\":[") != json) {
            __atomic_add_fetch(&bad_answers, 1, __ATOMIC_SEQ_CST);
        }
        TixFree(json);
    }
    return NULL;
}

static void *start_second(void *arg) {
    char *json = TixStart((char *)arg);
    long long h = handle_of(json);
    if (h <= 0) {
        __atomic_add_fetch(&bad_answers, 1, __ATOMIC_SEQ_CST);
    } else {
        TixFree(TixStop(h));
    }
    TixFree(json);
    return NULL;
}

int main(int argc, char **argv) {
    if (argc != 3) {
        fprintf(stderr, "usage: harness FIRST.db SECOND.db\n");
        return 2;
    }
    char options[4096];
    char second[4096];
    snprintf(options, sizeof options, "{\"db_path\":\"%s\",\"init_if_missing\":true}", argv[1]);
    snprintf(second, sizeof second, "{\"db_path\":\"%s\",\"init_if_missing\":true}", argv[2]);

    show("version", TixVersion());
    show("bad-json", TixStart((char *)"{not json"));
    show("public", TixStart((char *)"{\"db_path\":\"x.db\",\"listen\":\"0.0.0.0:0\"}"));

    char *started = TixStart(options);
    shared = handle_of(started);
    show("start", started);
    show("create", TixCreateTenant(shared, (char *)"{\"key\":\"work\",\"name\":\"Work\"}"));
    show("tenants", TixTenants(shared));
    show("token", TixToken(shared, (char *)"{\"tenant\":\"work\",\"name\":\"app\",\"ttl\":\"720h\"}"));
    show("bad-ttl", TixToken(shared, (char *)"{\"name\":\"x\",\"ttl\":\"soon\"}"));
    show("unknown", TixTenants(987654321));

    pthread_t threads[9];
    for (int i = 0; i < 8; i++) {
        pthread_create(&threads[i], NULL, list_tenants, NULL);
    }
    pthread_create(&threads[8], NULL, start_second, second);
    for (int i = 0; i < 9; i++) {
        pthread_join(threads[i], NULL);
    }
    printf("concurrent-bad %d\n", bad_answers);
    fflush(stdout);

    char line[16];
    if (fgets(line, sizeof line, stdin) == NULL) {
        return 1;
    }
    show("stop", TixStop(shared));
    show("stop-again", TixStop(shared));
    TixFree(NULL);
    return 0;
}
