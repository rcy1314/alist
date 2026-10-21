package _123Open

import (
	"context"

	"github.com/alist-org/alist/v3/internal/driver"
	"github.com/alist-org/alist/v3/internal/model"
	pan123 "github.com/okatu-loli/go-123pan"
)

type Open123 struct {
	model.Storage
	Addition
	tokenState

	client *pan123.Client
}

func (d *Open123) Config() driver.Config {
	return config
}

func (d *Open123) GetAddition() driver.Additional {
	return &d.Addition
}

func (d *Open123) Init(ctx context.Context) error {
	if d.UploadThread <= 0 {
		d.UploadThread = 3
	}
	if d.ValidDuration <= 0 {
		d.ValidDuration = 30
	}
	client, err := d.newSDKClient()
	if err != nil {
		return err
	}
	d.client = client
	if err := d.ensureToken(ctx); err != nil {
		return err
	}
	// verify the credentials so a misconfigured storage fails at mount time
	_, err = d.client.User.Info(ctx)
	return err
}

func (d *Open123) Drop(ctx context.Context) error {
	d.client = nil
	return nil
}

var (
	_ driver.Driver       = (*Open123)(nil)
	_ driver.MkdirResult  = (*Open123)(nil)
	_ driver.MoveResult   = (*Open123)(nil)
	_ driver.RenameResult = (*Open123)(nil)
	_ driver.Copy         = (*Open123)(nil)
	_ driver.Remove       = (*Open123)(nil)
	_ driver.PutResult    = (*Open123)(nil)
	_ driver.Other        = (*Open123)(nil)
)
