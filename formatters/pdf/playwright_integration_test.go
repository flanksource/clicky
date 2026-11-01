//go:build integration

package pdf_test

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/flanksource/clicky/formatters/pdf"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPlaywrightConversion(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Playwright SVG conversion")
}

var _ = DescribeTable("Playwright renders SVG with the installed driver", func(format, encoding string) {
	const width, height = 80, 60
	Expect(os.MkdirAll(filepath.Join("..", "..", ".tmp"), 0o755)).To(Succeed())
	dir, err := os.MkdirTemp(filepath.Join("..", "..", ".tmp"), "playwright-conversion-")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	source := filepath.Join(dir, "rectangle.svg")
	Expect(os.WriteFile(source, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="80" height="60"><rect width="80" height="60" fill="red"/></svg>`), 0o600)).To(Succeed())
	converter := pdf.NewPlaywrightConverter()
	DeferCleanup(converter.Close)
	output := filepath.Join(dir, "rectangle."+format)
	Expect(converter.Convert(context.Background(), source, output, &pdf.ConvertOptions{
		Format: format, Width: width, Height: height, Quality: 90,
	})).To(Succeed())
	content, err := os.ReadFile(output)
	Expect(err).NotTo(HaveOccurred())
	if format == "pdf" {
		Expect(bytes.HasPrefix(content, []byte("%PDF-"))).To(BeTrue())
		Expect(bytes.Contains(content, []byte("%%EOF"))).To(BeTrue())
		return
	}
	decoded, actualEncoding, err := image.Decode(bytes.NewReader(content))
	Expect(err).NotTo(HaveOccurred())
	Expect(actualEncoding).To(Equal(encoding))
	Expect(decoded.Bounds()).To(Equal(image.Rect(0, 0, width, height)))
	red, green, blue, _ := decoded.At(width/2, height/2).RGBA()
	Expect(red).To(BeNumerically(">", green+blue))
},
	Entry("PNG", "png", "png"),
	Entry("JPG", "jpg", "jpeg"),
	Entry("JPEG", "jpeg", "jpeg"),
	Entry("PDF", "pdf", ""),
)
