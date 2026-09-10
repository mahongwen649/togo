package generation

import (
	"fmt"

	"imgtool/server/internal/channels"
)

type EchoProvider struct{}

func (EchoProvider) GenerateText(snapshot channels.Snapshot, request TextRequest) (string, error) {
	return fmt.Sprintf("已接收图生文请求：%s / %s / %s", snapshot.ChannelName, snapshot.ModelID, request.Prompt), nil
}

func (EchoProvider) GenerateImage(snapshot channels.Snapshot, request ImageRequest) ([]ImageResult, error) {
	return nil, fmt.Errorf("当前供应源暂未实现文生图：%s / %s", snapshot.ChannelName, snapshot.ModelID)
}
