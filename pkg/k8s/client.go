package k8s

import (
	"context"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	conn *kubernetes.Clientset
}

func NewClient() *Client {
	home, err := os.UserHomeDir()

	if err != nil {
		log.Error("failed to get user home directory", "error", err)
		return nil
	}

	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))

	if err != nil {
		log.Error("failed to build kubernetes config", "error", err)
		return nil
	}

	conn, err := kubernetes.NewForConfig(config)

	if err != nil {
		log.Error("failed to create kubernetes client", "error", err)
		return nil
	}

	return &Client{
		conn: conn,
	}
}

func (client *Client) Deploy(
	configMap []byte,
	secret []byte,
	deployment []byte,
	service []byte,
) error {
	namespace := "agents"

	_, err := client.conn.CoreV1().ConfigMaps(namespace).Create(
		context.Background(),
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "agents-config",
			},
			Data: map[string]string{
				"config.yaml": string(configMap),
			},
		},
		metav1.CreateOptions{},
	)

	if err != nil {
		log.Error("failed to create deployment", "error", err)
		return err
	}

	_, err = client.conn.CoreV1().Secrets(namespace).Create(
		context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "agents-secret",
			},
			Data: map[string][]byte{
				"config.yaml": secret,
			},
		},
		metav1.CreateOptions{},
	)

	if err != nil {
		log.Error("failed to create secret", "error", err)
		return err
	}

	_, err = client.conn.AppsV1().Deployments(namespace).Create(
		context.Background(),
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "agents-deployment",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &[]int32{1}[0],
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "agents"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "agents",
								Image: "agents:latest",
								EnvFrom: []corev1.EnvFromSource{
									{
										ConfigMapRef: &corev1.ConfigMapEnvSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "agents-config",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		metav1.CreateOptions{},
	)

	if err != nil {
		log.Error("failed to create deployment", "error", err)
		return err
	}

	return nil
}
