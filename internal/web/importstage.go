// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// importStageTTL is how long an uploaded file waits between the steps of an
// import before it is dropped.
const importStageTTL = 2 * time.Hour

// maxImportStages bounds the uploads one server holds at once, so a reader
// who uploads and walks away a thousand times cannot fill the disk.
const maxImportStages = 200

// importStage is one upload between the steps of an import: the file on disk
// and what the steps have chosen so far. It belongs to the actor who uploaded
// it and to nobody else.
type importStage struct {
	ID       string
	TenantID string
	ActorID  string
	Project  string
	Filename string
	Format   core.TaskFormat
	Mapping  string
	Mode     core.TaskImportMode
	Lines    string
	Result   *core.TaskImportResult
	created  time.Time
	path     string
}

// importStages holds the uploads of imports in progress. The files live in a
// temporary directory of this process, so the steps of one import are served
// by the server that took the upload.
type importStages struct {
	mu    sync.Mutex
	dir   string
	items map[string]*importStage
	now   func() time.Time
}

func newImportStages() *importStages {
	return &importStages{items: map[string]*importStage{}, now: time.Now}
}

// put stores an upload for its actor and returns the stage.
func (s *importStages) put(actor *core.Actor, project, filename string, r io.Reader) (*importStage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	if len(s.items) >= maxImportStages {
		return nil, core.Invalid("too many imports are in progress on this server; try again in a while")
	}
	if s.dir == "" {
		dir, err := os.MkdirTemp("", "tix-import-")
		if err != nil {
			return nil, core.Internal("staging the upload: %v", err)
		}
		s.dir = dir
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, core.Internal("staging the upload: %v", err)
	}
	st := &importStage{ID: hex.EncodeToString(raw), TenantID: actor.TenantID, ActorID: actor.ID,
		Project: project, Filename: filename, created: s.now()}
	st.path = filepath.Join(s.dir, st.ID)
	f, err := os.OpenFile(st.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, core.Internal("staging the upload: %v", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(st.path)
		return nil, core.Invalid("reading the upload: %v", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(st.path)
		return nil, core.Internal("staging the upload: %v", err)
	}
	s.items[st.ID] = st
	return st, nil
}

// get returns the actor's own stage of a project. Anybody else's, an expired
// one or one for another project reads as not found.
func (s *importStages) get(actor *core.Actor, project, id string) (*importStage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	st, ok := s.items[id]
	if !ok || st.TenantID != actor.TenantID || st.ActorID != actor.ID || st.Project != project {
		return nil, core.NotFound("this import has expired or is not yours; upload the file again")
	}
	return st, nil
}

// open reads a stage's file.
func (s *importStages) open(st *importStage) (io.ReadCloser, error) {
	f, err := os.Open(st.path)
	if err != nil {
		return nil, core.NotFound("this import has expired; upload the file again")
	}
	return f, nil
}

// drop forgets a stage and removes its file.
func (s *importStages) drop(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.items[id]; ok {
		_ = os.Remove(st.path)
		delete(s.items, id)
	}
}

// expire drops every stage past its time. The caller holds the lock.
func (s *importStages) expire() {
	now := s.now()
	for id, st := range s.items {
		if now.Sub(st.created) > importStageTTL {
			_ = os.Remove(st.path)
			delete(s.items, id)
		}
	}
}
