package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

type Subscriber struct {
	listeners []chan a2a.Task
}

/*
Conn provides a connection to an S3-compatible storage service.
It uses MinIO client for better compatibility with MinIO server.
*/
type Conn struct {
	client      *minio.Client
	subscribers sync.Map
}

type ConnOption func(*Conn)

/*
NewConn creates a new S3 connection with default settings.
For a real implementation, this would configure MinIO credentials and endpoint.
*/
func NewConn(opts ...ConnOption) *Conn {
	conn := &Conn{}

	for _, opt := range opts {
		opt(conn)
	}

	return conn
}

/*
List retrieves a list of objects from S3 storage.
*/
func (conn *Conn) List(
	ctx context.Context, bucketName string,
) ([]minio.ObjectInfo, error) {
	objects := make([]minio.ObjectInfo, 0)

	for object := range conn.client.ListObjects(
		ctx, bucketName, minio.ListObjectsOptions{},
	) {
		if object.Err != nil {
			log.Error("failed to list objects", "error", object.Err)
			return nil, object.Err
		}

		objects = append(objects, object)
	}

	return objects, nil
}

/*
Get retrieves an object from S3 storage.
*/
func (conn *Conn) Get(
	ctx context.Context,
	bucketName string,
	objectKey string,
) (*bytes.Buffer, error) {
	object, err := conn.client.GetObject(
		ctx, bucketName, objectKey, minio.GetObjectOptions{},
	)

	if err != nil {
		log.Error("failed to get object", "error", err)
		return nil, err
	}

	defer object.Close()

	buf := &bytes.Buffer{}

	if _, err := io.Copy(buf, object); err != nil {
		log.Error("failed to copy object", "error", err)
		return nil, err
	}

	return buf, nil
}

/*
Put stores an object in S3 storage.
*/
func (conn *Conn) Put(
	ctx context.Context,
	bucketName string,
	objectKey string,
	body io.Reader,
) error {
	_, err := conn.client.PutObject(
		ctx, bucketName, objectKey, body, -1, minio.PutObjectOptions{},
	)

	return err
}

/*
ListenForObjectChanges starts listening for changes to a specific object in a bucket.
It returns a channel that will receive notifications for the object.
*/
func (conn *Conn) ListenForObjectChanges(
	ctx context.Context,
	bucketName string,
) error {
	events := []string{
		string(notification.ObjectCreatedPut),
		string(notification.ObjectCreatedCompleteMultipartUpload),
		string(notification.ObjectRemovedDelete),
		string(notification.ObjectCreatedPutTagging),
	}

	listener := conn.client.ListenBucketNotification(
		ctx,
		bucketName,
		"",
		"",
		events,
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case notificationInfo, ok := <-listener:
				if !ok {
					return
				}

				if notificationInfo.Err != nil {
					log.Error(
						"error receiving notification for bucket %s: %v",
						bucketName,
						notificationInfo.Err,
					)
					continue
				}

				for _, obj := range notificationInfo.Records {
					subscribers, ok := conn.subscribers.Load(obj.S3.Object.Key)

					if !ok {
						continue
					}

					buf, err := conn.Get(ctx, bucketName, obj.S3.Object.Key)

					if err != nil {
						log.Error("error getting object %s: %v", obj.S3.Object.Key, err)
						continue
					}

					task := a2a.Task{}

					if err := json.Unmarshal(buf.Bytes(), &task); err != nil {
						log.Error("error unmarshalling object %s: %v", obj.S3.Object.Key, err)
						continue
					}

					for _, subscriber := range subscribers.([]chan a2a.Task) {
						subscriber <- task
					}
				}
			}
		}
	}()

	return nil
}

func WithClient(client *minio.Client) ConnOption {
	return func(conn *Conn) {
		conn.client = client
	}
}
