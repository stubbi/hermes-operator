package oci

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

// ErrNoMatchingTag is returned when no tag in a list satisfies the requested channel.
var ErrNoMatchingTag = errors.New("no tag matches channel")

// Registry abstracts an OCI registry tag-listing API.
type Registry interface {
	ListTags(ctx context.Context, repository string) ([]string, error)
}

type cachedTags struct {
	tags    []string
	fetched time.Time
}

// Client is the production Registry implementation, backed by go-containerregistry.
type Client struct {
	mu    sync.Mutex
	cache map[string]cachedTags
	ttl   time.Duration
}

func NewClient(ttl time.Duration) *Client {
	return &Client{cache: map[string]cachedTags{}, ttl: ttl}
}

func (c *Client) ListTags(ctx context.Context, repository string) ([]string, error) {
	c.mu.Lock()
	cached, ok := c.cache[repository]
	c.mu.Unlock()
	if ok && c.ttl > 0 && time.Since(cached.fetched) < c.ttl {
		return cached.tags, nil
	}

	ref, err := name.NewRepository(repository)
	if err != nil {
		return nil, fmt.Errorf("parse repository %q: %w", repository, err)
	}

	tags, err := remote.List(ref, remote.WithContext(ctx))
	if err != nil {
		var terr *transport.Error
		if errors.As(err, &terr) && terr.StatusCode == 304 && ok {
			return cached.tags, nil
		}
		return nil, fmt.Errorf("list tags for %q: %w", repository, err)
	}

	c.mu.Lock()
	c.cache[repository] = cachedTags{tags: tags, fetched: time.Now()}
	c.mu.Unlock()
	return tags, nil
}

// HighestMatching returns the highest semver tag in `tags` that satisfies the channel.
// Non-semver tags are silently skipped. Per Masterminds/semver semantics,
// prereleases are NOT included in a range unless explicitly opted into.
func HighestMatching(tags []string, channel string) (string, error) {
	constraint, err := semver.NewConstraint(channel)
	if err != nil {
		return "", fmt.Errorf("invalid channel %q: %w", channel, err)
	}

	var versions []*semver.Version
	for _, t := range tags {
		v, err := semver.NewVersion(t)
		if err != nil {
			continue
		}
		if constraint.Check(v) {
			versions = append(versions, v)
		}
	}
	if len(versions) == 0 {
		return "", ErrNoMatchingTag
	}
	sort.Sort(semver.Collection(versions))
	return versions[len(versions)-1].Original(), nil
}

// DefaultChannel returns "<major>.x" for the given current tag, or "*" if not semver.
func DefaultChannel(currentTag string) string {
	v, err := semver.NewVersion(currentTag)
	if err != nil {
		return "*"
	}
	return fmt.Sprintf("%d.x", v.Major())
}
