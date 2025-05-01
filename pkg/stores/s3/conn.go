package s3

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Conn struct {
	Client     *s3.Client
	Uploader   *manager.Uploader
	Downloader *manager.Downloader
}

// NewConn creates a new S3 connection with default settings.
// For a real implementation, this would configure AWS credentials and region.
func NewConn() *Conn {
	// For the example, we'll create a minimal implementation
	// that doesn't actually connect to S3 but provides the interface
	return &Conn{}
}

func (conn *Conn) List(
	ctx context.Context, bucketName string,
) ([]types.Object, error) {
   // If no S3 client, fallback to local filesystem storage
   if conn.Client == nil {
       var objects []types.Object
       root := bucketName

       // Walk through local directory structure under bucketName
       _ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
           if err != nil {
               return err
           }
           if !d.IsDir() {
               rel, err := filepath.Rel(root, path)
               if err != nil {
                   return err
               }
               objects = append(objects, types.Object{Key: aws.String(rel)})
           }
           return nil
       })
       return objects, nil
   }
   var (
		err     error
		output  *s3.ListObjectsV2Output
		objects []types.Object
	)

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	}

	objectPaginator := s3.NewListObjectsV2Paginator(conn.Client, input)

	for objectPaginator.HasMorePages() {
		output, err = objectPaginator.NextPage(ctx)

		if err != nil {
			var noBucket *types.NoSuchBucket

			if errors.As(err, &noBucket) {
				log.Printf("Bucket %s does not exist.\n", bucketName)
				err = noBucket
			}

			break
		}

		objects = append(objects, output.Contents...)
	}

	return objects, err
}

func (conn *Conn) Get(
	ctx context.Context,
	bucketName string,
	objectKey string,
) (*bytes.Buffer, error) {
   // If no S3 client, fallback to local filesystem storage
   if conn.Client == nil {
       path := filepath.Join(bucketName, objectKey)
       data, err := os.ReadFile(path)
       if err != nil {
           return nil, err
       }
       return bytes.NewBuffer(data), nil
   }

	var (
		err    error
		output *s3.GetObjectOutput
	)

	input := &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}

	output, err = conn.Client.GetObject(ctx, input)

	if err != nil {
		return nil, err
	}

	defer output.Body.Close()

	buf := &bytes.Buffer{}

	if _, err = io.Copy(buf, output.Body); err != nil {
		return nil, err
	}

	return buf, nil
}

func (conn *Conn) Put(
	ctx context.Context,
	bucketName string,
	objectKey string,
	body io.Reader,
) error {
   // If no S3 client, fallback to local filesystem storage
   if conn.Client == nil {
       // Ensure directory exists
       fsPath := filepath.Join(bucketName, objectKey)
       if err := os.MkdirAll(filepath.Dir(fsPath), os.ModePerm); err != nil {
           return err
       }
       f, err := os.Create(fsPath)
       if err != nil {
           return err
       }
       defer f.Close()
       if _, err := io.Copy(f, body); err != nil {
           return err
       }
       return nil
   }

	var err error

	input := &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   body,
	}

	_, err = conn.Client.PutObject(ctx, input)

	return err
}
