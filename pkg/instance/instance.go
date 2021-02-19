package instance

import (
	"fmt"
	"encoding/json"
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
	ID uuid.UUID `json:"id"`
	SourceImagePath string `json:"sourceImagePath"`
	CompositeImageUrl string `json:"compositeImageUrl"`
	SourceImageWidth int `json:"sourceImageWidth"`
	SourceImageHeight int `json:"sourceImageHeight"`
	StepCountX int `json:"stepCountX"`
	StepCountY int `json:"stepCountY"`
	StepSizeX int `json:"stepSizeX"`
	StepSizeY int `json:"stepSizeY"`
}

func New(sourceImagePath string) (*Instance, error) {
	instance := &Instance{
		ID: uuid.New(),
		SourceImagePath: sourceImagePath,
		StepCountX: 6,
		StepCountY: 6,
	}

	err := instance.readSourceImageAttributes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read source image attributes")
	}

	instance.CompositeImageUrl=fmt.Sprintf("/v1/instance/%v/composite", instance.ID)
	err = instance.EnsurePath()
	if err != nil {
		return nil, errors.Wrap(err, "failed to ensure instance path")
	}
	err = instance.save()
	if err != nil {
		return nil, errors.Wrap(err, "failed to save instance data")
	}


	return instance, nil
}

func Open(instanceID string) (*Instance, error) {
	instanceUUID, err := uuid.Parse(instanceID)
	if err != nil {
		return nil, err
	}
	i := &Instance{
		ID: instanceUUID,
	}
	err = i.load()
	if err != nil {
		return nil, err
	}
	return i, err
}

func (i *Instance) readSourceImageAttributes() error {
	reader, err := os.Open(i.SourceImagePath)
	if err != nil {
		return errors.Wrapf(err, "Failed to open %s for reading", i.SourceImagePath)
	}
	source, _, err := image.Decode(reader)
	if err != nil {
		return errors.Wrap(err, "Failed to decode source image")
	}

	log.Infof("Source image bounds: min: %d,%d max: %d,%d", source.Bounds().Min.X, source.Bounds().Min.Y, source.Bounds().Max.X, source.Bounds().Max.Y)
	i.SourceImageWidth = source.Bounds().Max.X - source.Bounds().Min.X
	i.SourceImageHeight = source.Bounds().Max.Y - source.Bounds().Min.Y
	i.StepSizeX = i.SourceImageWidth / i.StepCountX
	i.StepSizeY = i.SourceImageHeight / i.StepCountY
	log.Infof("New instance: width=%d, height=%d, stepSizeX=%d, stepSizeY=%d", i.SourceImageWidth, i.SourceImageHeight, i.StepSizeX, i.StepSizeY)
	return nil
}

func (i *Instance) save() error {
	filePath := path.Join("data","instances",i.ID.String(),"instance")
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return errors.Wrap(err, "Couldn't open instance data file for writing")
	}
	defer f.Close()

	json, _ := json.MarshalIndent(i, "", " ")
	_, err = f.Write(json)
	if err != nil {
		return errors.Wrap(err, "Failed to write json to file")
	}

	return nil
}

func (i *Instance) load() error {
	instance := &Instance{}

	filePath := path.Join("data","instances",i.ID.String(),"instance")

	if _, err := os.Stat(filePath); err != nil {
		return nil
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return errors.Wrap(err, "Failed to load instance file")
	}

	err = json.Unmarshal(data, &instance)
	if err != nil {
		return errors.Wrap(err, "Failed to unmarshall instance data file")
	}

	i.SourceImagePath = instance.SourceImagePath
	i.SourceImageWidth = instance.SourceImageWidth
	i.SourceImageHeight = instance.SourceImageHeight
	i.StepCountX = instance.StepCountX
	i.StepCountY = instance.StepCountY
	i.StepSizeX = instance.StepSizeX
	i.StepSizeY = instance.StepSizeY

	log.Infof("Loaded instance %v", i.ID)
	return nil
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

	if _, err := os.Stat(instanceTilesPath); err != nil {
		err = os.MkdirAll(instanceTilesPath, 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create tiles folder for instance")
		}
	}

	tiles, err := ioutil.ReadDir(instanceTilesPath)
	if err != nil {
		return errors.Wrap(err, "Failed to list subfolders for instance")
	}

	contributions := make([]string,0)

	for _, f := range tiles {
		if !f.IsDir() {
			if path.Ext(f.Name()) == ".jpg" {
				contributions=append(contributions, f.Name())
			}
		}
	}

	reader, err := os.Open(i.SourceImagePath)
	if err != nil {
		return errors.Wrapf(err, "Failed to open %s for reading", i.SourceImagePath)
	}
	source, _, err := image.Decode(reader)
	if err != nil {
		return errors.Wrap(err, "Failed to decode source image")
	}

	newImageSize := image.Rect(source.Bounds().Min.X,source.Bounds().Min.Y,source.Bounds().Max.X,source.Bounds().Max.Y)
	stitchedImage := image.NewRGBA(newImageSize)

	for tY := 0; tY < i.StepCountY; tY++ {
		for tX := 0; tX < i.StepCountX; tX++ {
			offsetX := i.StepSizeX * tX
			offsetY := i.StepSizeY * tY
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
				for y := 0; y < i.StepSizeY; y++ {
					for x := 0; x < i.StepSizeX; x++ {
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

	if _, err := os.Stat(instanceTilesPath); err != nil && os.IsNotExist(err) {
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