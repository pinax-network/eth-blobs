package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Config points at any S3-compatible endpoint (AWS, Ceph RGW, MinIO, R2).
// Credentials come from the standard AWS environment variables or the default
// credential chain; they are deliberately not part of this struct so they
// never end up in a config file.
type S3Config struct {
	Endpoint  string        `yaml:"endpoint" json:"endpoint" mapstructure:"endpoint"`
	Bucket    string        `yaml:"bucket" json:"bucket" mapstructure:"bucket" validate:"required"`
	Prefix    string        `yaml:"prefix" json:"prefix" mapstructure:"prefix"`
	Region    string        `yaml:"region" json:"region" mapstructure:"region"`
	PathStyle bool          `yaml:"path_style" json:"path_style" mapstructure:"path_style"`
	Timeout   time.Duration `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
}

const (
	defaultRegion  = "us-east-1"
	defaultTimeout = 30 * time.Second
	deleteBatch    = 1000
)

// S3Store implements Store on top of the AWS SDK v2 S3 client.
type S3Store struct {
	client  *s3.Client
	bucket  string
	prefix  string
	timeout time.Duration
}

var _ Store = (*S3Store)(nil)

// NewS3 builds a client for cfg. Endpoint empty means AWS proper; a non-empty
// endpoint with PathStyle is what Ceph RGW and MinIO want.
func NewS3(ctx context.Context, cfg S3Config) (*S3Store, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("s3: bucket is required")
	}
	region := cfg.Region
	if region == "" {
		region = defaultRegion
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("s3: load aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.PathStyle
		// Ceph RGW and MinIO do not return the integrity checksums the SDK adds by default;
		// without this every GET logs "Response has no supported checksum".
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	prefix := strings.Trim(cfg.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	return &S3Store{client: client, bucket: cfg.Bucket, prefix: prefix, timeout: timeout}, nil
}

func (s *S3Store) key(k string) string { return s.prefix + k }

func (s *S3Store) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s.timeout)
}

func (s *S3Store) get(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.key(key))})
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%s: %w", key, ErrNotFound)
		}
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	defer func() { _ = out.Body.Close() }()
	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", key, err)
	}
	return data, nil
}

func (s *S3Store) put(ctx context.Context, key string, data []byte, contentType string) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.key(key)),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

func (s *S3Store) GetSlot(ctx context.Context, slot uint64) ([]byte, error) {
	return s.get(ctx, SlotKey(slot))
}

func (s *S3Store) PutSlot(ctx context.Context, slot uint64, data []byte) error {
	return s.put(ctx, SlotKey(slot), data, "application/x-protobuf")
}

func (s *S3Store) DeleteSlots(ctx context.Context, slots []uint64) error {
	for start := 0; start < len(slots); start += deleteBatch {
		end := start + deleteBatch
		if end > len(slots) {
			end = len(slots)
		}
		objects := make([]types.ObjectIdentifier, 0, end-start)
		for _, slot := range slots[start:end] {
			objects = append(objects, types.ObjectIdentifier{Key: aws.String(s.key(SlotKey(slot)))})
		}
		ctx, cancel := s.withTimeout(ctx)
		out, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
		})
		cancel()
		if err != nil {
			return fmt.Errorf("delete slots: %w", err)
		}
		for _, e := range out.Errors {
			if e.Code != nil && *e.Code == "NoSuchKey" {
				continue
			}
			return fmt.Errorf("delete %s: %s %s", aws.ToString(e.Key), aws.ToString(e.Code), aws.ToString(e.Message))
		}
	}
	return nil
}

func (s *S3Store) GetRoot(ctx context.Context, root string) (uint64, error) {
	data, err := s.get(ctx, RootKey(root))
	if err != nil {
		return 0, err
	}
	slot, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("root %s: malformed slot %q: %w", root, string(data), err)
	}
	return slot, nil
}

func (s *S3Store) PutRoot(ctx context.Context, root string, slot uint64) error {
	return s.put(ctx, RootKey(root), []byte(strconv.FormatUint(slot, 10)), "text/plain")
}

func (s *S3Store) GetHead(ctx context.Context) (Head, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	var best Head
	found := false
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.key(headPrefix)),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return Head{}, fmt.Errorf("list heads: %w", err)
		}
		for _, obj := range page.Contents {
			key := strings.TrimPrefix(aws.ToString(obj.Key), s.prefix)
			data, err := s.get(ctx, key)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					continue // raced with a writer rewriting it
				}
				return Head{}, err
			}
			var head Head
			if err := json.Unmarshal(data, &head); err != nil {
				return Head{}, fmt.Errorf("head %s: %w", key, err)
			}
			if !found || head.Slot > best.Slot {
				best, found = head, true
			}
		}
	}
	if !found {
		return Head{}, fmt.Errorf("head: %w", ErrNotFound)
	}
	return best, nil
}

func (s *S3Store) PutHead(ctx context.Context, writer string, head Head) error {
	head.Writer = writer
	data, err := json.Marshal(head)
	if err != nil {
		return err
	}
	return s.put(ctx, HeadKey(writer), data, "application/json")
}

func (s *S3Store) GetCursor(ctx context.Context, writer string) (string, error) {
	data, err := s.get(ctx, CursorKey(writer))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *S3Store) PutCursor(ctx context.Context, writer string, cursor string) error {
	return s.put(ctx, CursorKey(writer), []byte(cursor), "text/plain")
}

func isNotFound(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return true
		}
	}
	return false
}
