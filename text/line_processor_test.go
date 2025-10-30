package text_test

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/flanksource/clicky/text"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLineProcessor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LineProcessor Suite")
}

var _ = Describe("LineFilter", func() {
	Context("with single processor", func() {
		It("should process complete lines", func() {
			buf := &bytes.Buffer{}

			uppercase := func(line string) (string, bool) {
				return strings.ToUpper(line), false
			}

			writer := text.LineFilter(buf, uppercase)

			_, err := writer.Write([]byte("hello\nworld\n"))
			Expect(err).ToNot(HaveOccurred())

			Expect(buf.String()).To(Equal("HELLO\nWORLD\n"))
		})

		It("should skip lines when processor returns skip=true", func() {
			buf := &bytes.Buffer{}

			skipAll := func(line string) (string, bool) {
				return line, true
			}

			writer := text.LineFilter(buf, skipAll)

			_, err := writer.Write([]byte("hello\nworld\n"))
			Expect(err).ToNot(HaveOccurred())

			Expect(buf.String()).To(BeEmpty())
		})

		It("should handle partial lines across writes", func() {
			buf := &bytes.Buffer{}

			uppercase := func(line string) (string, bool) {
				return strings.ToUpper(line), false
			}

			writer := text.LineFilter(buf, uppercase)

			_, err := writer.Write([]byte("hel"))
			Expect(err).ToNot(HaveOccurred())
			Expect(buf.String()).To(BeEmpty()) // No newline yet

			_, err = writer.Write([]byte("lo\nwor"))
			Expect(err).ToNot(HaveOccurred())
			Expect(buf.String()).To(Equal("HELLO\n"))

			_, err = writer.Write([]byte("ld\n"))
			Expect(err).ToNot(HaveOccurred())
			Expect(buf.String()).To(Equal("HELLO\nWORLD\n"))
		})
	})

	Context("with multiple processors", func() {
		It("should execute processors left-to-right", func() {
			buf := &bytes.Buffer{}

			addPrefix := func(line string) (string, bool) {
				return "[PREFIX] " + line, false
			}

			addSuffix := func(line string) (string, bool) {
				return line + " [SUFFIX]", false
			}

			writer := text.LineFilter(buf, addPrefix, addSuffix)

			_, err := writer.Write([]byte("test\n"))
			Expect(err).ToNot(HaveOccurred())

			Expect(buf.String()).To(Equal("[PREFIX] test [SUFFIX]\n"))
		})

		It("should short-circuit when first processor skips", func() {
			buf := &bytes.Buffer{}

			skipFirst := func(line string) (string, bool) {
				return line, true
			}

			neverCalled := func(line string) (string, bool) {
				Fail("Second processor should not be called")
				return line, false
			}

			writer := text.LineFilter(buf, skipFirst, neverCalled)

			_, err := writer.Write([]byte("test\n"))
			Expect(err).ToNot(HaveOccurred())

			Expect(buf.String()).To(BeEmpty())
		})

		It("should short-circuit when middle processor skips", func() {
			buf := &bytes.Buffer{}
			callCount := 0

			count := func(line string) (string, bool) {
				callCount++
				return line, false
			}

			skip := func(line string) (string, bool) {
				return line, true
			}

			neverCalled := func(line string) (string, bool) {
				Fail("Third processor should not be called")
				return line, false
			}

			writer := text.LineFilter(buf, count, skip, neverCalled)

			_, err := writer.Write([]byte("test\n"))
			Expect(err).ToNot(HaveOccurred())

			Expect(callCount).To(Equal(1))
			Expect(buf.String()).To(BeEmpty())
		})
	})

	Context("with concurrent writes", func() {
		It("should be thread-safe", func() {
			buf := &bytes.Buffer{}

			uppercase := func(line string) (string, bool) {
				return strings.ToUpper(line), false
			}

			writer := text.LineFilter(buf, uppercase)

			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()
					_, err := writer.Write([]byte("line\n"))
					Expect(err).ToNot(HaveOccurred())
				}(i)
			}

			wg.Wait()

			lines := strings.Split(buf.String(), "\n")
			lines = lines[:len(lines)-1] // Remove last empty line
			Expect(len(lines)).To(Equal(10))
			for _, line := range lines {
				Expect(line).To(Equal("LINE"))
			}
		})
	})

	Context("with panic recovery", func() {
		It("should recover from panic and skip line", func() {
			buf := &bytes.Buffer{}

			panicProcessor := func(line string) (string, bool) {
				panic("test panic")
			}

			writer := text.LineFilter(buf, panicProcessor)

			_, err := writer.Write([]byte("test\n"))
			Expect(err).ToNot(HaveOccurred())

			Expect(buf.String()).To(BeEmpty())
		})

		It("should continue processing after panic", func() {
			buf := &bytes.Buffer{}

			panicOnFirst := func(line string) (string, bool) {
				if line == "panic" {
					panic("test panic")
				}
				return line, false
			}

			writer := text.LineFilter(buf, panicOnFirst)

			_, err := writer.Write([]byte("panic\n"))
			Expect(err).ToNot(HaveOccurred())
			Expect(buf.String()).To(BeEmpty())

			_, err = writer.Write([]byte("normal\n"))
			Expect(err).ToNot(HaveOccurred())
			Expect(buf.String()).To(Equal("normal\n"))
		})

		It("should not call remaining processors after panic", func() {
			buf := &bytes.Buffer{}

			panicProcessor := func(line string) (string, bool) {
				panic("test panic")
			}

			neverCalled := func(line string) (string, bool) {
				Fail("Should not be called after panic")
				return line, false
			}

			writer := text.LineFilter(buf, panicProcessor, neverCalled)

			_, err := writer.Write([]byte("test\n"))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
