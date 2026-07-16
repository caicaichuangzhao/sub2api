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

func TestPrepareOpenAIImagesNonStreamingResponsePreservesUpstreamImageBytes(t *testing.T) {
	encoded := encodeOpenAIImageDimensionsTestPNG(t, 941, 1672)
	body := []byte(`{"created":1,"data":[{"b64_json":"` + encoded + `","output_format":"png"}],"size":"941x1672","output_format":"png"}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	result, err := (&OpenAIGatewayService{}).prepareOpenAIImagesNonStreamingResponse(
		resp,
		c,
		"b64_json",
		"",
		"1088x1440",
	)
	require.NoError(t, err)
	require.Equal(t, body, result.Body)
	require.Equal(t, encoded, gjson.GetBytes(result.Body, "data.0.b64_json").String())
	require.Equal(t, "941x1672", detectOpenAIImageResultSize(gjson.GetBytes(result.Body, "data.0.b64_json").String()))
}

func TestHandleOpenAIImagesOAuthNonStreamingResponsePreservesUpstreamImageBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	encoded := encodeOpenAIImageDimensionsTestPNG(t, 941, 1672)
	stream := "data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000001,\"usage\":{\"input_tokens\":5,\"output_tokens\":9},\"tools\":[{\"type\":\"image_generation\",\"model\":\"gpt-image-2\",\"size\":\"auto\",\"output_format\":\"png\"}],\"output\":[{\"type\":\"image_generation_call\",\"result\":\"" + encoded + "\",\"output_format\":\"png\"}]}}\n\n" +
		"data: [DONE]\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(stream)),
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)

	_, count, sizes, err := (&OpenAIGatewayService{}).handleOpenAIImagesOAuthNonStreamingResponse(
		resp,
		c,
		"b64_json",
		"gpt-image-2",
		"1088x1440",
	)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, []string{"941x1672"}, sizes)
	require.Equal(t, "941x1672", gjson.Get(recorder.Body.String(), "size").String())
	require.Equal(t, encoded, gjson.Get(recorder.Body.String(), "data.0.b64_json").String())
}

func TestNormalizeOpenAIImageBase64KeepsRequiredPadding(t *testing.T) {
	for _, encoded := range []string{"AQ==", "AQI="} {
		require.Equal(t, encoded, normalizeOpenAIImageBase64(encoded))
	}
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
