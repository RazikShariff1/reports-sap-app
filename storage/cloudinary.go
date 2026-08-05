// Package storage uploads generated report files to Cloudinary via its
// signed upload REST API, so reports never touch local disk.
package storage

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // required by Cloudinary's signing scheme, not used for security
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	cloudName string
	apiKey    string
	apiSecret string
)

// Init parses CLOUDINARY_URL (cloudinary://<api_key>:<api_secret>@<cloud_name>).
func Init() error {
	raw := os.Getenv("CLOUDINARY_URL")

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse CLOUDINARY_URL: %w", err)
	}

	if u.Scheme != "cloudinary" || u.User == nil || u.Host == "" {
		return fmt.Errorf("CLOUDINARY_URL must be of the form cloudinary://<api_key>:<api_secret>@<cloud_name>")
	}

	secret, ok := u.User.Password()
	if !ok {
		return fmt.Errorf("CLOUDINARY_URL missing api_secret")
	}

	cloudName = u.Host
	apiKey = u.User.Username()
	apiSecret = secret

	return nil
}

// UploadCSV uploads r as a raw asset under publicID and returns its secure URL.
func UploadCSV(ctx context.Context, r io.Reader, publicID string) (string, error) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	signature := sign(map[string]string{
		"public_id": publicID,
		"timestamp": timestamp,
	})

	var body bytes.Buffer

	w := multipart.NewWriter(&body)

	fields := map[string]string{
		"public_id": publicID,
		"timestamp": timestamp,
		"api_key":   apiKey,
		"signature": signature,
	}

	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return "", fmt.Errorf("write field %s: %w", k, err)
		}
	}

	part, err := w.CreateFormFile("file", publicID+".csv")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, r); err != nil {
		return "", fmt.Errorf("copy file contents: %w", err)
	}

	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close multipart writer: %w", err)
	}

	uploadURL := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/raw/upload", cloudName)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	var out struct {
		SecureURL string `json:"secure_url"`
		Error     struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode cloudinary response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cloudinary upload failed: %s", out.Error.Message)
	}

	return out.SecureURL, nil
}

// sign implements Cloudinary's signed-upload scheme: sort params, join as
// key=value pairs, append the api secret, then SHA-1 the result.
func sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	var sb strings.Builder

	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}

		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(params[k])
	}

	sb.WriteString(apiSecret)

	sum := sha1.Sum([]byte(sb.String()))

	return hex.EncodeToString(sum[:])
}
