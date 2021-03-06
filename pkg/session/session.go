package session

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/andrewmyhre/donk-server/pkg/instance"
	"github.com/andrewmyhre/donk-server/pkg/tile"
	"github.com/google/uuid"
	"image/jpeg"
	"io/ioutil"
	"os"
	"path"
	log "github.com/sirupsen/logrus"
	"github.com/pkg/errors"
	"strings"

	"image"
	_ "image/jpeg"
)

type Session struct {
	ID uuid.UUID `json:"id"`
	Instance *instance.Instance `json:"instance"`
	Location tile.Location `json:"location"`
	BackgroundImage image.Image `json:omit`
}

type sessionOut struct {
	ID uuid.UUID `json:"id"`
	InstanceID uuid.UUID `json:"instanceID"`
	Location tile.Location `json:"location"`
}

func NewSession(instance *instance.Instance, x,y int) (*Session,error) {
	session := &Session {
		Instance: instance,
		ID: uuid.New(),
		Location: tile.Location {
			X: x,
			Y: y,
		},
	}

	err := session.initializeBackgroundImage()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize session background image")
	}


	err = session.save()
	if err != nil {
		log.Warn(errors.Wrap(err, "Failed to save session"))
	}

	return session, nil
}

func (s *Session) save() error {
	out := sessionOut{
		ID: s.ID,
		InstanceID: s.Instance.ID,
		Location: s.Location,
	}

	filePath := path.Join("data","instances",s.Instance.ID.String(),"sessions",s.ID.String(),"session")
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return errors.Wrap(err, "Couldn't open session data file for writing")
	}
	defer f.Close()

	json, _ := json.MarshalIndent(out, "", " ")
	_, err = f.Write(json)
	if err != nil {
		return errors.Wrap(err, "Failed to write json to file")
	}

	return nil
}

func (s *Session) load() error {
	out := &sessionOut {}

	filePath := path.Join("data","instances",s.Instance.ID.String(),"sessions",s.ID.String(),"session")

	if _, err := os.Stat(filePath); err != nil {
		return nil
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return errors.Wrap(err, "Failed to load session file")
	}

	err = json.Unmarshal(data, &out)
	if err != nil {
		return errors.Wrap(err, "Failed to unmarshall session data file")
	}

	s.Location.X=out.Location.X
	s.Location.Y=out.Location.Y
	s.Instance.ID = out.InstanceID
	log.Infof("Loaded session for %d,%d", s.Location.X, s.Location.Y)
	return nil
}

func Open(instance *instance.Instance, sessionID string) (*Session, error) {
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, errors.Wrap(err, sessionID + " is not a valid session ID")
	}

	session := &Session {
		ID: sessionUUID,
		Instance: instance,
	}
	err = session.load()
	if err != nil {
		return nil, errors.Wrap(err,"Couldn't open session")
	}

	return session, nil
}

func (s *Session) initializeBackgroundImage() error {
	sessionPath := path.Join("data", "instances", s.Instance.ID.String(), "sessions",s.ID.String())
	backgroundImagePath := path.Join(sessionPath,"background.jpg")
	tileFilename := fmt.Sprintf("%d,%d.jpg", s.Location.X, s.Location.Y)
	tileImage := path.Join("data", "instances", s.Instance.ID.String(), "tiles", tileFilename)

	if _, err := os.Stat(sessionPath); err != nil && os.IsNotExist(err) {
		err := os.MkdirAll(sessionPath, 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create session folder")
		}
		log.Infof("Created path for session %v", s.ID)
	}

	if _, err := os.Stat(tileImage); err == nil {
		log.Infof("Using existing tile image for %d,%d", s.Location.X, s.Location.Y)
		input, err := ioutil.ReadFile(tileImage)
        if err != nil {
                return errors.Wrap(err, "Failed to read tile image")
        }

        err = ioutil.WriteFile(backgroundImagePath, input, 0644)
        if err != nil {
                return errors.Wrap(err, "Failed to write tile to background image")
        }
		return nil
	}
	log.Info("No existing tile, generating a background from source")
	
	reader, err := os.Open(s.Instance.SourceImagePath)
	if err != nil {
	    return errors.Wrapf(err, "Failed to open %s for reading", s.Instance.SourceImagePath)
	}
	source, _, err := image.Decode(reader)
	if err != nil {
		return errors.Wrap(err, "Failed to decode source image")
	}

	newImageSize := image.Rect(0,0,s.Instance.StepSizeX,s.Instance.StepSizeY)
	newImage := image.NewRGBA(newImageSize)

	sourceX0 := s.Location.X * s.Instance.StepSizeX
	sourceY0 := s.Location.Y * s.Instance.StepSizeY

	for y := 0; y < s.Instance.StepSizeY; y++ {
		for x := 0; x < s.Instance.StepSizeX; x++ {
			newImage.Set(x,y,source.At(sourceX0+x,sourceY0+y))
		}
	}

	writer, err := os.Create(backgroundImagePath)
	if err != nil {
		return errors.Wrap(err, "Failed to open background image for writing")
	}

	err = jpeg.Encode(writer, newImage, &jpeg.Options{
		Quality: 100,
	})

	if err != nil {
		return errors.Wrap(err, "Failed to write background image")
	}
	return nil
}

func (s *Session) ReadBackgroundImage() ([]byte,error) {
	backgroundImagePath := path.Join("data", "instances", s.Instance.ID.String(), "sessions", s.ID.String(), "background.jpg")
	dat, err := ioutil.ReadFile(backgroundImagePath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read session background image")
	}
	return dat, nil
}

func (s *Session) UpdateBackgroundImage(data []byte) error {
	outFilePath := path.Join("data", "instances", s.Instance.ID.String(), "sessions", s.ID.String(), "background.jpg")
	encodedImageData := strings.Replace(string(data), "data:image/jpeg;base64,", "", 1)
	decodedImageData, err := base64.StdEncoding.DecodeString(encodedImageData)

	if err != nil {
		return errors.Wrapf(err, "Failed to decode from base64: %s", encodedImageData)
	}

	imageFile, err := os.OpenFile(outFilePath,os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return errors.Wrap(err, "Couldn't open image file for writing")
	}
	defer imageFile.Close()

	_, err = imageFile.Write(decodedImageData)
	if err != nil {
		if err != nil {
			return errors.Wrap(err, "Couldn't write image data")
		}
	}

	log.Infof("Saved %s", outFilePath)

	err = s.Instance.UpdateTile(s.Location, decodedImageData)
	if err != nil {
		return errors.Wrap(err, "Failed to update instance tile")
	}
	return nil
}

func Find(sessionID string) (*Session, error) {
	instancesPath := path.Join("data", "instances")

	instances, err := ioutil.ReadDir(instancesPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list subfolders for instance")
	}

	for _, instanceID := range instances {
		if instanceID.IsDir() {
			instanceUUID, err := uuid.Parse(instanceID.Name())
			if err != nil {
				continue
			}
			// TODO: load the instance from disk if available
			instanceObj := &instance.Instance {
				ID: instanceUUID,
			}
			sessionsPath := path.Join(instancesPath, instanceID.Name(), "sessions")
			if _, err := os.Stat(sessionsPath); err != nil {
				continue
			}
			sessions, err := ioutil.ReadDir(sessionsPath)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to list subfolders for instance " + instanceID.Name())
			}
			for _, s := range sessions {
				if s.IsDir() {
					_, err := uuid.Parse(s.Name())
					if err != nil {
						continue
					}
					session, err := Open(instanceObj, sessionID)
					if err != nil {
						return nil, errors.Wrap(err, "Failed to open session " + session.ID.String())
					}
					return session, nil
				}
			}
		}
	}

	return nil, errors.New("Failed to find session " + sessionID)
}