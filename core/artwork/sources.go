package artwork

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/dhowden/tag"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/core/ffmpeg"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/resources"
)

type sourceFunc func() (io.ReadCloser, string, error)

func (f sourceFunc) String() string {
	name := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	name = strings.TrimPrefix(name, "github.com/navidrome/navidrome/core.")
	name = strings.TrimPrefix(name, "(*artwork).")
	name = strings.TrimSuffix(name, ".func1")
	return name
}

func (a *artwork) fromAlbum(ctx context.Context, id model.ArtworkID) sourceFunc {
	return func() (io.ReadCloser, string, error) {
		r, path, err := a.get(ctx, id, 0)
		if err != nil {
			return nil, "", err
		}
		return r, path, nil
	}
}

func fromExternalFile(ctx context.Context, files string, pattern string) sourceFunc {
	return func() (io.ReadCloser, string, error) {
		for _, file := range filepath.SplitList(files) {
			_, name := filepath.Split(file)
			match, err := filepath.Match(pattern, strings.ToLower(name))
			if err != nil {
				log.Warn(ctx, "Error matching cover art file to pattern", "pattern", pattern, "file", file)
				continue
			}
			if !match {
				continue
			}
			f, err := os.Open(file)
			if err != nil {
				log.Warn(ctx, "Could not open cover art file", "file", file, err)
				continue
			}
			return f, file, err
		}
		return nil, "", fmt.Errorf("pattern '%s' not matched by files %v", pattern, files)
	}
}

func fromTag(path string) sourceFunc {
	return func() (io.ReadCloser, string, error) {
		if path == "" {
			return nil, "", nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, "", err
		}
		defer f.Close()

		m, err := tag.ReadFrom(f)
		if err != nil {
			return nil, "", err
		}

		picture := m.Picture()
		if picture == nil {
			return nil, "", fmt.Errorf("no embedded image found in %s", path)
		}
		return io.NopCloser(bytes.NewReader(picture.Data)), path, nil
	}
}

func fromFFmpegTag(ctx context.Context, ffmpeg ffmpeg.FFmpeg, path string) sourceFunc {
	return func() (io.ReadCloser, string, error) {
		if path == "" {
			return nil, "", nil
		}
		r, err := ffmpeg.ExtractImage(ctx, path)
		if err != nil {
			return nil, "", err
		}
		defer r.Close()
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, r)
		if err != nil {
			return nil, "", err
		}
		return io.NopCloser(buf), path, nil
	}
}

func fromPlaceholder() sourceFunc {
	return func() (io.ReadCloser, string, error) {
		r, _ := resources.FS().Open(consts.PlaceholderAlbumArt)
		return r, consts.PlaceholderAlbumArt, nil
	}
}