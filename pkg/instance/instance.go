package instance

import (
	"fmt"
	"github.com/andrewmyhre/donk-server/pkg/tile"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"image"
	"image/jpeg"
	"io/ioutil"
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

func (i *Instance) GetStitchedImage() ([]byte, error) {
	imageDataPath := path.Join("data", "instances", i.ID.String(), "stitch.jpg")
	imageData, err := ioutil.ReadFile(imageDataPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read image file")
	}
	return imageData, nil
}

func (i *Instance) StitchSessionImage() error {
	instanceDataPath := path.Join("data", "instances", i.ID.String())
	instanceTilesPath := path.Join("data", "instances", i.ID.String(), "tiles")

	tiles, err := ioutil.ReadDir(instanceTilesPath)
	if err != nil {
		return errors.Wrap(err, "Failed to list subfolders for instance")
	}

	contributions := make([]string,0)

	for _, f := range tiles {
		log.Infof("%s", f.Name())
		if !f.IsDir() {
			if path.Ext(f.Name()) == ".jpg" {
				contributions=append(contributions, f.Name())
			}
		}
	}

	reader, err := os.Open("assets/paper4.jpg")
	if err != nil {
		return errors.Wrap(err, "Failed to open assets/paper4.jpg for reading")
	}
	source, _, err := image.Decode(reader)
	if err != nil {
		return errors.Wrap(err, "Failed to decode source image")
	}

	newImageSize := image.Rect(source.Bounds().Min.X,source.Bounds().Min.Y,source.Bounds().Max.X,source.Bounds().Max.Y)
	stitchedImage := image.NewRGBA(newImageSize)

	for tY := 0; tY < 6; tY++ {
		for tX := 0; tX < 6; tX++ {
			offsetX := tile.XStep * tX
			offsetY := tile.YStep * tY
			tileImagePath := path.Join(instanceTilesPath, fmt.Sprintf("%d,%d.jpg", tX, tY))
			if _, err := os.Stat(tileImagePath); err == nil {
				contrReader, err := os.Open(tileImagePath)
				if err != nil {
					log.Warn(errors.Wrap(err, "failed to open contribution"))
					continue
				}

				contrImage, _, err := image.Decode(contrReader)
				if err != nil {
					log.Warn(errors.Wrap(err, "failed to decode contribution"))
					continue
				}

				bounds := contrImage.Bounds()
				for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
					for x := bounds.Min.X; x < bounds.Max.X; x++ {
						stitchedImage.Set(offsetX+x,offsetY+y,contrImage.At(x,y))
					}
				}
			} else {
				for y := 0; y < tile.YStep; y++ {
					for x := 0; x < tile.XStep; x++ {
						stitchedImage.Set(offsetX+x, offsetY+y, source.At(offsetX+x, offsetY+y))
					}
				}
			}
		}
	}

	stitchedImageFilename := path.Join(instanceDataPath,"stitch.jpg")
	stitchedImageWriter, err := os.OpenFile(stitchedImageFilename,os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return errors.Wrap(err, "Couldn't open image file for writing")
	}
	defer stitchedImageWriter.Close()
	err = jpeg.Encode(stitchedImageWriter, stitchedImage, &jpeg.Options{
		Quality: 90,
	})

	log.Infof("Saved %s", stitchedImageFilename)


	return nil
}

func (i *Instance) UpdateTile(location tile.Location, imageData []byte) error {
	fileName := fmt.Sprintf("%d,%d.jpg", location.X, location.Y)
	instanceTilesPath := path.Join("data", "instances", i.ID.String(), "tiles")
	outFilePath := path.Join(instanceTilesPath, fileName)

	if stat, err := os.Stat(instanceTilesPath); err != nil || !stat.IsDir() {
		err := os.MkdirAll(instanceTilesPath, 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create path for tiles")
		}
	}

	imageFile, err := os.OpenFile(outFilePath,os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return errors.Wrap(err, "Couldn't open image file for writing")
	}
	defer imageFile.Close()

	_, err = imageFile.Write(imageData)
	if err != nil {
		if err != nil {
			return errors.Wrap(err, "Couldn't write image data")
		}
	}

	log.Infof("Saved %s", outFilePath)

	err = i.StitchSessionImage()
	if err != nil {
		return errors.Wrap(err, "Couldn't update instance stitch image")
	}

	return nil
}