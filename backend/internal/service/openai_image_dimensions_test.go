package service

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAIImageBase64DimensionsFitsWithoutCropping(t *testing.T) {
	encoded := encodeOpenAIImageDimensionsTestPNG(t, 941, 1672)

	normalized, format, ok := normalizeOpenAIImageBase64Dimensions(encoded, "png", "1088x1440")
	require.True(t, ok)
	require.Equal(t, "png", format)
	requireOpenAIImageDimensions(t, normalized, 1088, 1440)
}

func TestNormalizeOpenAIImageBase64DimensionsPreservesTopAndBottomEdges(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 100, 200))
	for y := 0; y < 200; y++ {
		fill := color.NRGBA{G: 255, A: 255}
		if y < 20 {
			fill = color.NRGBA{R: 255, A: 255}
		} else if y >= 180 {
			fill = color.NRGBA{B: 255, A: 255}
		}
		for x := 0; x < 100; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	var source bytes.Buffer
	require.NoError(t, png.Encode(&source, img))

	normalized, _, ok := normalizeOpenAIImageBase64Dimensions(
		base64.StdEncoding.EncodeToString(source.Bytes()),
		"png",
		"200x200",
	)
	require.True(t, ok)
	raw, err := base64.StdEncoding.DecodeString(normalized)
	require.NoError(t, err)
	result, err := png.Decode(bytes.NewReader(raw))
	require.NoError(t, err)

	require.Equal(t, color.NRGBA{R: 255, A: 255}, color.NRGBAModel.Convert(result.At(100, 0)))
	require.Equal(t, color.NRGBA{B: 255, A: 255}, color.NRGBAModel.Convert(result.At(100, 199)))
}

func TestNormalizeOpenAIImageBase64KeepsRequiredPadding(t *testing.T) {
	for _, encoded := range []string{"AQ==", "AQI="} {
		require.Equal(t, encoded, normalizeOpenAIImageBase64(encoded))
	}
}

func TestNormalizeOpenAIImagesResponseBodyDimensionsOverridesAutoMetadata(t *testing.T) {
	encoded := encodeOpenAIImageDimensionsTestPNG(t, 1122, 1402)
	body := []byte(`{"created":1,"data":[{"b64_json":"` + encoded + `","output_format":"png"}],"size":"auto","output_format":"png"}`)

	normalized, ok := normalizeOpenAIImagesResponseBodyDimensions(body, "1088x1440")
	require.True(t, ok)
	require.Equal(t, "1088x1440", gjson.GetBytes(normalized, "size").String())
	requireOpenAIImageDimensions(t, gjson.GetBytes(normalized, "data.0.b64_json").String(), 1088, 1440)
}

func TestHandleOpenAIImagesOAuthNonStreamingResponseNormalizesRequestedSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	encoded := encodeOpenAIImageDimensionsTestPNG(t, 941, 1672)
	stream := "data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000001,\"usage\":{\"input_tokens\":5,\"output_tokens\":9},\"tools\":[{\"type\":\"image_generation\",\"model\":\"gpt-image-2-codex\",\"size\":\"auto\",\"output_format\":\"png\"}],\"output\":[{\"type\":\"image_generation_call\",\"result\":\"" + encoded + "\",\"output_format\":\"png\"}]}}\n\n" +
		"data: [DONE]\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(stream)),
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	c.Request.Host = "api.example.com"

	svc := &OpenAIGatewayService{}
	_, count, sizes, err := svc.handleOpenAIImagesOAuthNonStreamingResponse(resp, c, "url", "gpt-image-2", "1088x1440")
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, []string{"1088x1440"}, sizes)
	require.Equal(t, "1088x1440", gjson.Get(recorder.Body.String(), "size").String())

	imageURL := gjson.Get(recorder.Body.String(), "data.0.url").String()
	id := strings.TrimPrefix(imageURL, "http://api.example.com"+generatedImageURLPathPrefix)
	require.NotEqual(t, imageURL, id)
	asset, ok := loadGeneratedImage(id)
	require.True(t, ok)
	requireOpenAIImageBytesDimensions(t, asset.Data, 1088, 1440)
}

func encodeOpenAIImageDimensionsTestPNG(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x % 255), G: uint8(y % 255), B: 120, A: 255})
		}
	}
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, img))
	return base64.StdEncoding.EncodeToString(buffer.Bytes())
}

func requireOpenAIImageDimensions(t *testing.T, encoded string, width, height int) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	requireOpenAIImageBytesDimensions(t, raw, width, height)
}

func requireOpenAIImageBytesDimensions(t *testing.T, raw []byte, width, height int) {
	t.Helper()
	config, _, err := image.DecodeConfig(bytes.NewReader(raw))
	require.NoError(t, err)
	require.Equal(t, width, config.Width)
	require.Equal(t, height, config.Height)
}
