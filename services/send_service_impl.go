package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/structs"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/h2non/bimg"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
	"net/http"
	"os"
	"time"
)

type SendServiceImpl struct {
	WaCli *whatsmeow.Client
}

func NewSendService(waCli *whatsmeow.Client) SendService {
	return &SendServiceImpl{
		WaCli: waCli,
	}
}

func (service SendServiceImpl) SendText(c *fiber.Ctx, request structs.SendMessageRequest) (response structs.SendMessageResponse, err error) {
	recipient, ok := utils.ParseJID(request.PhoneNumber)
	if !ok {
		return response, errors.New("invalid JID " + request.PhoneNumber)
	}
	msg := &waProto.Message{Conversation: proto.String(request.Message)}
	ts, err := service.WaCli.SendMessage(recipient, "", msg)
	if err != nil {
		return response, err
	} else {
		response.Status = fmt.Sprintf("Message sent (server timestamp: %s)", ts)
	}
	return response, nil
}

func (service SendServiceImpl) SendImage(c *fiber.Ctx, request structs.SendImageRequest) (response structs.SendImageResponse, err error) {
	// Resize image
	oriImagePath := fmt.Sprintf("%s/%s", config.PathSendImage, request.Image.Filename)
	err = c.SaveFile(request.Image, oriImagePath)
	if err != nil {
		return response, err
	}
	openImageBuffer, err := bimg.Read(oriImagePath)
	newImage, err := bimg.NewImage(openImageBuffer).Process(bimg.Options{Quality: 90, Width: 400, Height: 400, Embed: true})
	if err != nil {
		return response, err
	}

	newImagePath := fmt.Sprintf("%s/new-%s", config.PathSendImage, request.Image.Filename)
	err = bimg.Write(newImagePath, newImage)
	if err != nil {
		return response, err
	}

	removeFile := func(paths ...string) {
		time.Sleep(5 * time.Second)
		for _, path := range paths {
			err := os.Remove(path)
			if err != nil {
				fmt.Println("error when delete " + path)
			}
		}

	}

	// Send to WA server
	dataWaCaption := request.Caption
	dataWaRecipient, ok := utils.ParseJID(request.PhoneNumber)
	if !ok {
		return response, errors.New("invalid JID " + request.PhoneNumber)
	}
	dataWaImage, err := os.ReadFile(newImagePath)
	if err != nil {
		return response, err
	}
	uploadedImage, err := service.WaCli.Upload(context.Background(), dataWaImage, whatsmeow.MediaImage)
	if err != nil {
		fmt.Printf("Failed to upload file: %v", err)
		return response, err
	}

	msg := &waProto.Message{ImageMessage: &waProto.ImageMessage{
		Caption:       proto.String(dataWaCaption),
		Url:           proto.String(uploadedImage.URL),
		DirectPath:    proto.String(uploadedImage.DirectPath),
		MediaKey:      uploadedImage.MediaKey,
		Mimetype:      proto.String(http.DetectContentType(dataWaImage)),
		FileEncSha256: uploadedImage.FileEncSHA256,
		FileSha256:    uploadedImage.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(dataWaImage))),
		ViewOnce:      proto.Bool(request.ViewOnce),
	}}
	ts, err := service.WaCli.SendMessage(dataWaRecipient, "", msg)
	if err != nil {
		return response, err
	} else {
		go removeFile(oriImagePath, newImagePath)
		response.Status = fmt.Sprintf("Image message sent (server timestamp: %s)", ts)
		return response, nil
	}
}
