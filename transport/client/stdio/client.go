package stdio

import (
	"context"
	"fmt"
	"github.com/viant/gosh/runner"
	"github.com/viant/gosh/runner/local"
	"github.com/viant/gosh/runner/ssh"
	"github.com/viant/jsonrpc"
	transport2 "github.com/viant/jsonrpc/transport"
	"github.com/viant/jsonrpc/transport/client/base"
	"github.com/viant/scy/cred/secret"
	cssh "golang.org/x/crypto/ssh"
	"strings"
	"time"
)

// Client represent a base
type Client struct {
	base      *base.Client
	client    runner.Runner
	secret    secret.Resource
	sshConfig *cssh.ClientConfig
	host      string
	command   string
	args      []string
	env       map[string]string
	ctx       context.Context
}

func (c *Client) start(ctx context.Context) error {
	if err := c.ensureSSHConfig(ctx); err != nil {
		return err // ensure SSH config is set up before initializing the service
	}
	var options = []runner.Option{
		runner.AsPipeline(),
	}
	if c.sshConfig != nil {
		c.client = ssh.New(c.host, c.sshConfig, options...) // create a new SSH client with the provided SSH config
	} else {
		c.client = local.New(options...) // fallback to local client if no SSH config is provided
	}
	c.base.Transport = &Transport{client: c.client}
	cmd := c.command
	if len(c.args) > 0 {
		cmd = fmt.Sprintf("%s %s", c.command, strings.Join(c.args, " "))
	}
	go c.startCommand(ctx, cmd)
	return nil
}

func (c *Client) startCommand(ctx context.Context, cmd string) {
	output, code, err := c.client.Run(ctx, cmd, runner.WithEnvironment(c.env), runner.WithListener(c.stdoutListener()))
	if err != nil {
		c.base.SetError(err)
	}
	if code != -1 {
		c.base.SetError(fmt.Errorf("command exited with code: %d %v", code, output))
	}
}

func (c *Client) stdoutListener() runner.Listener {
	var builder strings.Builder
	return func(stdout string, hasMore bool) {
		index := strings.Index(stdout, "\n")
		if index != -1 {
			defer builder.Reset()
			builder.WriteString(stdout[:index])
			data := []byte(builder.String())
			c.base.HandleMessage(c.ctx, data)
			return

		} else {
			builder.WriteString(stdout)
		}
	}
}

func (c *Client) Notify(ctx context.Context, request *jsonrpc.Notification) error {
	return c.base.Notify(ctx, request)
}

func (c *Client) Send(ctx context.Context, request *jsonrpc.Request) (*jsonrpc.Response, error) {
	return c.base.Send(ctx, request)
}

func (c *Client) ensureSSHConfig(ctx context.Context) error {
	if c.sshConfig != nil || c.host == "" {
		return nil
	}
	if c.secret != "" {
		secrets := secret.New()
		cred, err := secrets.GetCredentials(ctx, string(c.secret))
		if err != nil {
			return err // unable to retrieve credentials for SSH config
		}
		c.sshConfig, err = cred.SSH.Config(ctx) // this will populate the SSH config from the secret
		// SSH config is required for remote connections, if host is specified but no sshConfig provided
		return err
	}
	return fmt.Errorf("sshConfig is required but not provided for host: %s", c.host)
}

func New(command string, options ...Option) (*Client, error) {
	c := &Client{
		command: command,
		ctx:     context.Background(),
		base: &base.Client{
			RoundTrips: transport2.NewRoundTrips(20),
			RunTimeout: 15 * time.Minute,
			Transport:  &Transport{},
			Handler:    &base.Handler{},
			Logger:     jsonrpc.DefaultLogger,
		},
	}
	for _, opt := range options {
		opt(c)
	}
	err := c.start(c.ctx)
	return c, err
}
