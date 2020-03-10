package klient

import (
	"bytes"
	"fmt"
	"io"

	v1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/validation"
)

// Client is a kubernetes client, like `kubectl`
type Client struct {
	Clientset        *kubernetes.Clientset
	factory          *factory
	validator        validation.Schema
	namespace        string
	enforceNamespace bool
}

// Result is an alias for the Kubernetes CLI runtime resource.Result
type Result = resource.Result

// BuilderOptions parameters to create a Resource Builder
type BuilderOptions struct {
	Unstructured bool
}

// DefaultBuilderOptions creates a BuilderOptions with the default values for
// the parameters to create a Resource Builder
func DefaultBuilderOptions() *BuilderOptions {
	return &BuilderOptions{
		Unstructured: true,
	}
}

// NewE creates a kubernetes client, returns an error if fail
func NewE(context, kubeconfig string) (*Client, error) {
	factory := newFactory(context, kubeconfig)

	// If `true` it will always validate the given objects/resources
	validator, _ := factory.Validator(true)

	namespace, enforceNamespace, err := factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		namespace = v1.NamespaceDefault
		enforceNamespace = true
	}
	clientset, err := factory.KubernetesClientSet()
	if err != nil {
		return nil, err
	}
	if clientset == nil {
		return nil, fmt.Errorf("cannot create a clientset from given context and kubeconfig")
	}

	return &Client{
		factory:          factory,
		Clientset:        clientset,
		validator:        validator,
		namespace:        namespace,
		enforceNamespace: enforceNamespace,
	}, nil
}

// New creates a kubernetes client
func New(context, kubeconfig string) *Client {
	client, _ := NewE(context, kubeconfig)
	return client
}

// Builder creates a resource builder
func (c *Client) builder(opt *BuilderOptions) *resource.Builder {
	if opt == nil {
		opt = DefaultBuilderOptions()
	}
	b := c.factory.NewBuilder()

	if opt.Unstructured {
		b = b.Unstructured()
	}

	return b.
		Schema(c.validator).
		ContinueOnError().
		NamespaceParam(c.namespace).DefaultNamespace()
}

// ResultForFilenameParam returns the builder results for the given list of files or URLs
func (c *Client) ResultForFilenameParam(filenames []string, opt *BuilderOptions) *Result {
	filenameOptions := &resource.FilenameOptions{
		Recursive: false,
		Filenames: filenames,
	}

	return c.builder(opt).
		FilenameParam(c.enforceNamespace, filenameOptions).
		Flatten().
		Do()
}

// ResultForReader returns the builder results for the given reader
func (c *Client) ResultForReader(r io.Reader, opt *BuilderOptions) *Result {
	return c.builder(opt).
		Stream(r, "").
		Flatten().
		Do()
}

// ResultForContent returns the builder results for the given content
func (c *Client) ResultForContent(content []byte, opt *BuilderOptions) *Result {
	b := bytes.NewBuffer(content)
	return c.ResultForReader(b, opt)
}

func failedTo(action string, info *resource.Info, err error) error {
	var resKind string
	if info.Mapping != nil {
		resKind = info.Mapping.GroupVersionKind.Kind + " "
	}

	return fmt.Errorf("cannot %s object Kind: %q,	Name: %q, Namespace: %q. %s", action, resKind, info.Name, info.Namespace, err)
}
