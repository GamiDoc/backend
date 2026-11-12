package objectstore

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type S3StoreConfig struct {
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
	BaseURL         string
}

type S3Store struct {
	client  *s3.Client
	bucket  string
	baseURL string
}

func NewS3Store(ctx context.Context, cfg S3StoreConfig) (*S3Store, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.UsePathStyle
		if strings.TrimSpace(cfg.Endpoint) != "" {
			options.EndpointResolver = s3.EndpointResolverFromURL(strings.TrimRight(cfg.Endpoint, "/"))
		}
	})

	return &S3Store{
		client:  client,
		bucket:  cfg.Bucket,
		baseURL: cfg.BaseURL,
	}, nil
}

func (s *S3Store) Save(ctx context.Context, key string, data []byte) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         stringPtr(trimLeftSlash(key)),
		Body:        bytes.NewReader(data),
		ContentType: stringPtr(contentTypeForKey(key)),
	})
	if err != nil {
		return "", err
	}

	return buildPublicURL(s.baseURL, key), nil
}

func (s *S3Store) Read(ctx context.Context, key string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    stringPtr(trimLeftSlash(key)),
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()

	return io.ReadAll(output.Body)
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    stringPtr(trimLeftSlash(key)),
	})
	return err
}

func (s *S3Store) KeyFromURL(url string) (string, bool) {
	base := trimRightSlash(s.baseURL)
	prefix := base + "/"
	if len(url) <= len(prefix) || url[:len(prefix)] != prefix {
		return "", false
	}
	return url[len(prefix):], true
}

func stringPtr(value string) *string {
	return &value
}

func (s *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    stringPtr(trimLeftSlash(key)),
	})
	if err == nil {
		return true, nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey") {
		return false, nil
	}
	return false, err
}
