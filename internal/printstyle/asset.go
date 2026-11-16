package printstyle

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxAssetBytes is the largest single image.
const MaxAssetBytes = 2 << 20

// Asset is a named image a style or a meet holds. It is referred to as
// asset:名称 in templates, CSS and Markdown.
type Asset struct {
	Name string
	MIME string
	Data []byte
}

// DataURI is the asset as an embedded image.
func (a Asset) DataURI() string {
	return "data:" + a.MIME + ";base64," + base64.StdEncoding.EncodeToString(a.Data)
}

// SniffImage names the image type of a file's first bytes: image/png,
// image/jpeg or image/webp; "" for anything else.
func SniffImage(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case bytes.HasPrefix(data, []byte("\xff\xd8\xff")):
		return "image/jpeg"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp"
	}
	return ""
}

// ValidateName checks an image's name: 1–40 characters, none of them white
// space, a quote or a bracket (what would end a reference to it).
func ValidateName(name string) error {
	n := utf8.RuneCountInString(name)
	if n < 1 || n > 40 {
		return errors.New("图片名称需要 1–40 个字")
	}
	if !utf8.ValidString(name) || strings.ContainsFunc(name, func(r rune) bool {
		return r <= ' ' || r == 0x7f || r == 0x85 || r == 0xA0 || r == 0x3000 || r == 0x2028 || r == 0x2029 || strings.ContainsRune("\"'`()[]{}<>（）【】《》“”‘’「」\\&%;,", r)
	}) {
		return errors.New("图片名称不能含空格、引号、括号或 & % ; ,")
	}
	return nil
}

// ValidateAsset accepts png, jpeg and webp images of at most 2 MB whose
// first bytes agree with the declared type.
func ValidateAsset(a Asset) error {
	if err := ValidateName(a.Name); err != nil {
		return err
	}
	if len(a.Data) == 0 {
		return fmt.Errorf("图片「%s」是空的", a.Name)
	}
	if len(a.Data) > MaxAssetBytes {
		return fmt.Errorf("图片「%s」超过 %d MB", a.Name, MaxAssetBytes>>20)
	}
	got := SniffImage(a.Data)
	if got == "" {
		return fmt.Errorf("图片「%s」不是 png、jpeg 或 webp", a.Name)
	}
	if a.MIME != got {
		return fmt.Errorf("图片「%s」声明的类型 %s 与内容（%s）不一致", a.Name, a.MIME, got)
	}
	return nil
}

// A transparent pixel stands in for a missing image so nothing breaks.
const blankImage = "data:image/gif;base64,R0lGODlhAQABAAAAACH5BAEKAAEALAAAAAABAAEAAAICTAEAOw=="

var assetRefRe = regexp.MustCompile(`(?i)(\bsrc\s*=\s*["']?|url\(\s*["']?)asset:([^\s"'()<>]+)`)

// ResolveAssets replaces asset:名称 in src attributes and in CSS url() by
// the image embedded as a data URI. A name that is not in assets is a
// warning and the reference becomes an empty image. Names written as
// percent-escapes (Markdown does that to non-ASCII addresses) are
// understood.
func ResolveAssets(doc string, assets map[string]Asset) (string, []string) {
	if !strings.Contains(doc, "asset:") {
		return doc, nil
	}
	var warns []string
	missing := map[string]bool{}
	out := assetRefRe.ReplaceAllStringFunc(doc, func(m string) string {
		sub := assetRefRe.FindStringSubmatch(m)
		prefix, name := sub[1], sub[2]
		a, ok := assets[name]
		if !ok {
			if un, err := url.PathUnescape(name); err == nil {
				name = un
				a, ok = assets[name]
			}
		}
		if ok && ValidateAsset(a) == nil {
			return prefix + a.DataURI()
		}
		if !missing[name] {
			missing[name] = true
			warns = append(warns, fmt.Sprintf("找不到图片「%s」，已去掉引用", name))
		}
		return prefix + blankImage
	})
	return out, warns
}
