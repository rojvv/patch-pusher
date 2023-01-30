package main

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type QueueItem struct {
	repository string
	branch     string
	patch      *multipart.FileHeader
}

var queue = make(chan QueueItem)

func main() {
	http.Handle("/", http.HandlerFunc(handleRequest))
	go worker()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
}

func handleRequest(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(5_000_000); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
	} else {
		var repository string
		var branch string
		var patch *multipart.FileHeader
		if values := r.MultipartForm.Value["repository"]; len(values) == 1 {
			repository = values[0]
		}
		if values := r.MultipartForm.Value["branch"]; len(values) == 1 {
			branch = values[0]
		}
		if values := r.MultipartForm.File["patch"]; len(values) == 1 {
			patch = r.MultipartForm.File["patch"][0]
		}
		if repository == "" || branch == "" || patch == nil {
			rw.WriteHeader(http.StatusBadRequest)
		} else {
			queue <- QueueItem{repository, branch, patch}
			log.Info().Msg("Queued a valid request.")
			rw.WriteHeader(http.StatusOK)
		}
	}
}

func worker() {
	for {
		item := <-queue
		logCtx := log.With().Str("repository", item.repository).Str("branch", item.branch).Logger()
		logCtx.Info().Msg("Started processing request.")
		cloneDir := uuid.NewString()
		cloneStart := time.Now()
		if err := execGitCommand("", "clone", nil, "--depth", "1", "--branch", item.branch, item.repository, cloneDir); err != nil {
			logCtx.Err(err).Msg("Failed to clone repository.")
			return
		}
		logCtx.Info().Dur("timeElapsed", time.Since(cloneStart)).Msg("Repository cloned.")
		file, err := item.patch.Open()
		if err != nil {
			logCtx.Err(err).Msg("Failed to open patch.")
			return
		}
		if err := execGitCommand(cloneDir, "am", file, "-"); err != nil {
			logCtx.Err(err).Msg("Failed to apply patch.")
			return
		}
		logCtx.Info().Msg("Patch applied.")
		pushStart := time.Now()
		if err := execGitCommand(cloneDir, "push", nil); err != nil {
			logCtx.Err(err).Msg("Failed to push changes.")
			return
		}
		logCtx.Info().Dur("timeElapsed", time.Since((pushStart))).Msg("Changes pushed.")
		if err := os.RemoveAll(cloneDir); err != nil {
			logCtx.Err(err).Msg("Failed to remove cloneDir.")
			return
		}
		logCtx.Info().Msg("Request served.")
	}
}

func execGitCommand(dir string, name string, file multipart.File, args ...string) error {
	args = append([]string{name, "--quiet"}, args...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	writer, err := cmd.StdinPipe()
	if err := cmd.Start(); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	if file != nil {
		if _, err := io.Copy(writer, file); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return cmd.Wait()
}
