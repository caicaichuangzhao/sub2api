package service

import (
	"bytes"
	"encoding/base64"
	"image"
	stddraw "image/draw"
	"image/jpeg"
	"image/png"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	openAIImageMaxRequestedEdge   = 3840
	openAIImageMaxRequestedPixels = 8_294_400
)

func parseOpenAIRequestedImageDimensions(size string) (int, int, bool) {
	widthRaw, heightRaw, ok := strings.Cut(strings.ToLower(strings.TrimSpace(size)), "x")
	if !ok {
		return 0, 0, false
	}
	width, err := strconv.Atoi(strings.TrimSpace(widthRaw))
	if err != nil {
		return 0, 0, false
	}
	height, err := strconv.Atoi(strings.TrimSpace(heightRaw))
	if err != nil {
		return 0, 0, false
	}
	if width <= 0 || height <= 0 || width > openAIImageMaxRequestedEdge || height > openAIImageMaxRequestedEdge {
		return 0, 0, false
	}
	if int64(width)*int64(height) > openAIImageMaxRequestedPixels {
		return 0, 0, false
	}
	return width, height, true
}

func normalizeOpenAIImageBase64Dimensions(encoded, outputFormat, requestedSize string) (string, string, bool) {
	targetWidth, targetHeight, ok := parseOpenAIRequestedImageDimensions(requestedSize)
	if !ok {
		return encoded, outputFormat, false
	}

	raw := strings.TrimSpace(encoded)
	if _, payload, dataURLOK := splitOpenAIImageDataURL(raw); dataURLOK {
		raw = payload
	}
	raw = normalizeOpenAIImageBase64(raw)
	if raw == "" {
		return encoded, outputFormat, false
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return encoded, outputFormat, false
	}
	src, decodedFormat, err := image.Decode(bytes.NewReader(decoded))
	if err != nil || src.Bounds().Empty() {
		return encoded, outputFormat, false
	}

	format := normalizeOpenAIImageResizeFormat(outputFormat, decodedFormat)
	if src.Bounds().Dx() == targetWidth && src.Bounds().Dy() == targetHeight {
		return raw, format, true
	}
	if format != "png" && format != "jpeg" {
		return encoded, outputFormat, false
	}

	crop := centeredOpenAIImageCrop(src.Bounds(), targetWidth, targetHeight)
	dst := image.NewNRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, stddraw.Src, nil)

	var out bytes.Buffer
	switch format {
	case "jpeg":
		if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 95}); err != nil {
			return encoded, outputFormat, false
		}
	default:
		if err := png.Encode(&out, dst); err != nil {
			return encoded, outputFormat, false
		}
	}
	return base64.StdEncoding.EncodeToString(out.Bytes()), format, true
}

func normalizeOpenAIImageResizeFormat(outputFormat, decodedFormat string) string {
	format := strings.ToLower(strings.TrimSpace(outputFormat))
	if slash := strings.LastIndex(format, "/"); slash >= 0 {
		format = format[slash+1:]
	}
	switch format {
	case "jpg":
		return "jpeg"
	case "png", "jpeg", "webp":
		return format
	}
	switch strings.ToLower(strings.TrimSpace(decodedFormat)) {
	case "jpg", "jpeg":
		return "jpeg"
	case "webp":
		return "webp"
	default:
		return "png"
	}
}

func centeredOpenAIImageCrop(bounds image.Rectangle, targetWidth, targetHeight int) image.Rectangle {
	sourceWidth := bounds.Dx()
	sourceHeight := bounds.Dy()
	crop := bounds
	if int64(sourceWidth)*int64(targetHeight) > int64(targetWidth)*int64(sourceHeight) {
		cropWidth := max(1, int(int64(sourceHeight)*int64(targetWidth)/int64(targetHeight)))
		crop.Min.X = bounds.Min.X + (sourceWidth-cropWidth)/2
		crop.Max.X = crop.Min.X + cropWidth
	} else if int64(sourceWidth)*int64(targetHeight) < int64(targetWidth)*int64(sourceHeight) {
		cropHeight := max(1, int(int64(sourceWidth)*int64(targetHeight)/int64(targetWidth)))
		crop.Min.Y = bounds.Min.Y + (sourceHeight-cropHeight)/2
		crop.Max.Y = crop.Min.Y + cropHeight
	}
	return crop
}

func normalizeOpenAIResponsesImageResultDimensions(results []openAIResponsesImageResult, requestedSize string) bool {
	if len(results) == 0 {
		return false
	}
	normalized := append([]openAIResponsesImageResult(nil), results...)
	for i := range normalized {
		encoded, format, ok := normalizeOpenAIImageBase64Dimensions(normalized[i].Result, normalized[i].OutputFormat, requestedSize)
		if !ok {
			return false
		}
		normalized[i].Result = encoded
		normalized[i].OutputFormat = format
		normalized[i].Size = strings.TrimSpace(requestedSize)
	}
	copy(results, normalized)
	return true
}

func normalizeOpenAIImagesResponseBodyDimensions(body []byte, requestedSize string) ([]byte, bool) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, false
	}
	if _, _, ok := parseOpenAIRequestedImageDimensions(requestedSize); !ok {
		return body, false
	}
	data := gjson.GetBytes(body, "data")
	if !data.IsArray() || len(data.Array()) == 0 {
		return body, false
	}

	out := append([]byte(nil), body...)
	for i, item := range data.Array() {
		itemPath := "data." + strconv.Itoa(i)
		outputFormat := firstNonEmptyString(
			item.Get("output_format").String(),
			item.Get("mime_type").String(),
			item.Get("content_type").String(),
			gjson.GetBytes(body, "output_format").String(),
		)
		field := "b64_json"
		encoded := strings.TrimSpace(item.Get(field).String())
		dataURL := false
		if encoded == "" {
			field = "url"
			encoded = strings.TrimSpace(item.Get(field).String())
			dataURL = isOpenAIImageDataURL(encoded)
			if !dataURL {
				return body, false
			}
		}

		normalized, format, ok := normalizeOpenAIImageBase64Dimensions(encoded, outputFormat, requestedSize)
		if !ok {
			return body, false
		}
		if dataURL {
			normalized = "data:" + openAIImageOutputMIMEType(format) + ";base64," + normalized
		}
		var err error
		out, err = sjson.SetBytes(out, itemPath+"."+field, normalized)
		if err != nil {
			return body, false
		}
		if item.Get("size").Exists() {
			out, _ = sjson.SetBytes(out, itemPath+".size", strings.TrimSpace(requestedSize))
		}
	}
	out, _ = sjson.SetBytes(out, "size", strings.TrimSpace(requestedSize))
	return out, true
}
