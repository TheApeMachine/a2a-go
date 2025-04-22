package s3

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log"

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

func (conn *Conn) List(
	ctx context.Context, bucketName string,
) ([]types.Object, error) {
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
) (io.ReadCloser, error) {
	buf := bytes.NewBuffer([]byte{})

	result, err := conn.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})

	if err != nil {
		var noKey *types.NoSuchKey

		if errors.As(err, &noKey) {
			err = noKey
		}

		return nil, err
	}

	defer result.Body.Close()

	_, err = io.Copy(buf, result.Body)

	return io.NopCloser(bufio.NewReader(buf)), err
}

func (conn *Conn) Put(
	ctx context.Context,
	bucketName string,
	objectKey string,
	body io.Reader,
) error {
	_, err := conn.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   body,
	})

	return err
}
