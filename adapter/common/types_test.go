package adaptercommon

import (
	"chat/utils"
	"testing"
)

func TestCreateChatPropsDropsSamplingControls(t *testing.T) {
	floatValue := float32(0.7)
	intValue := 5

	props := CreateChatProps(&ChatProps{
		Temperature:       &floatValue,
		TopP:              &floatValue,
		TopK:              &intValue,
		PresencePenalty:   &floatValue,
		FrequencyPenalty:  &floatValue,
		RepetitionPenalty: &floatValue,
	}, &utils.Buffer{})

	if props.Temperature != nil ||
		props.TopP != nil ||
		props.TopK != nil ||
		props.PresencePenalty != nil ||
		props.FrequencyPenalty != nil ||
		props.RepetitionPenalty != nil {
		t.Fatalf("sampling controls must be removed from chat props: %#v", props)
	}
}
