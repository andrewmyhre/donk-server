package instance

import (
	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
)

type Instance struct {
	ID uuid.UUID
}

func New() (*Instance, error) {
	instance := &Instance{ID: uuid.New()}

	return instance, nil
}

func (i *Instance) EnsurePath() error {
	p := path.Join("data", "instances", i.ID.String())
	if s, err := os.Stat(p); err != nil || !s.IsDir() {
		err := os.MkdirAll(p, 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create path for Instance")
		}
		log.Infof("Created path for instance %v", i.ID)
	}

	return nil
}
