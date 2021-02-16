package session

import (
	"encoding/base64"
	"github.com/andrewmyhre/donk-server/pkg/instance"
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

const xStep=924
const yStep=624

type Location struct {
	X int
	Y int
}

type Session struct {
	ID uuid.UUID
	Instance *instance.Instance
	Location Location
	BackgroundImage image.Image
}

func NewSession(instance *instance.Instance, x,y int) (*Session,error) {
	session := &Session {
		Instance: instance,
		ID: uuid.New(),
		Location: Location {
			X: x,
			Y: y,
		},
	}

	err := session.initializeBackgroundImage()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize session background image")
	}


	return session, nil
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

	return session, nil
}

func (s *Session) initializeBackgroundImage() error {
	sessionPath := path.Join("data", "instances", s.Instance.ID.String(), "sessions",s.ID.String())
	if stat, err := os.Stat(sessionPath); err != nil || !stat.IsDir() {
		err := os.MkdirAll(sessionPath, 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create session folder")
		}
		log.Infof("Created path for session %v", s.ID)
	}

	reader, err := os.Open("assets/paper4.jpg")
	if err != nil {
	    return errors.Wrap(err, "Failed to open assets/paper4.job for reading")
	}
	source, _, err := image.Decode(reader)
	if err != nil {
		return errors.Wrap(err, "Failed to decode source image")
	}

	newImageSize := image.Rect(0,0,xStep,yStep)
	newImage := image.NewRGBA(newImageSize)

	sourceX0 := s.Location.X * xStep
	sourceY0 := s.Location.Y * yStep

	for y := 0; y < yStep; y++ {
		for x := 0; x < xStep; x++ {
			newImage.Set(x,y,source.At(sourceX0+x,sourceY0+y))
		}
	}

	writer, err := os.Create(path.Join("data", sessionPath,"background.jpg"))
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

func (s *Session) SaveImage(data []byte) error {
	outFilePath := path.Join("data", "instances", s.Instance.ID.String(), "sessions", s.ID.String(), "image.jpg")
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

	return nil
}