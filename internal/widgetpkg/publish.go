package widgetpkg

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DefaultRegistry is the public npm registry.
const DefaultRegistry = "https://registry.npmjs.org"

// PublishOptions says where and how a package is published.
type PublishOptions struct {
	// Registry is the URL of the npm registry.
	Registry string
	// Token is the token of the registry. It goes into the Authorization
	// header only.
	Token string
	// Tag is the dist-tag of the version, for example latest.
	Tag string
	// Access is public or restricted, or "" for the rule of the registry.
	Access string
	Client *http.Client
}

// Publish sends one version of the package to an npm registry through its
// HTTP API (REQ-ISL-14). It runs no node and no npm.
func Publish(ctx context.Context, p Package, opt PublishOptions) error {
	meta, err := p.meta()
	if err != nil {
		return err
	}
	tarball, err := p.Tarball()
	if err != nil {
		return err
	}
	if opt.Token == "" {
		return errors.New("no token of the registry")
	}
	if opt.Access != "" && opt.Access != "public" && opt.Access != "restricted" {
		return fmt.Errorf("the access %q is not public or restricted", opt.Access)
	}
	base, err := url.Parse(opt.Registry)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return fmt.Errorf("the registry %q is not an http or https URL", opt.Registry)
	}
	registry := strings.TrimSuffix(base.String(), "/")
	tag := opt.Tag
	if tag == "" {
		tag = "latest"
	}
	file := TarballName(p.Name, p.Version)
	sum1 := sha1.Sum(tarball)
	sum512 := sha512.Sum512(tarball)
	// The version document is package.json and the facts of the tarball.
	var version map[string]any
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return err
	}
	version["_id"] = p.Name + "@" + p.Version
	version["dist"] = map[string]any{
		"integrity": "sha512-" + base64.StdEncoding.EncodeToString(sum512[:]),
		"shasum":    hex.EncodeToString(sum1[:]),
		"tarball":   registry + "/" + p.Name + "/-/" + file,
	}
	doc := map[string]any{
		"_id":       p.Name,
		"name":      p.Name,
		"dist-tags": map[string]string{tag: p.Version},
		"versions":  map[string]any{p.Version: version},
		"_attachments": map[string]any{
			file: map[string]any{
				"content_type": "application/octet-stream",
				"data":         base64.StdEncoding.EncodeToString(tarball),
				"length":       len(tarball),
			},
		},
	}
	if opt.Access != "" {
		doc["access"] = opt.Access
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	// The registry takes the name of a scope package with an escaped slash.
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, registry+"/"+strings.Replace(p.Name, "/", "%2f", 1), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+opt.Token)
	client := opt.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	// The registry says why in the error field of its answer.
	var answer struct {
		Error string `json:"error"`
	}
	text, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	_ = json.Unmarshal(text, &answer)
	if answer.Error != "" {
		return fmt.Errorf("the registry answered %d: %s", res.StatusCode, answer.Error)
	}
	return fmt.Errorf("the registry answered %d", res.StatusCode)
}
