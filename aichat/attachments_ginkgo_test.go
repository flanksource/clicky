package aichat

import (
	"context"
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	capapi "github.com/flanksource/captain/pkg/api"
)

type fakeAttachmentResolver struct{}

func (fakeAttachmentResolver) Resolve(_ context.Context, inputs []AttachmentInput) ([]capapi.AttachmentRef, error) {
	out := make([]capapi.AttachmentRef, len(inputs))
	for i, input := range inputs {
		out[i] = capapi.AttachmentRef{
			ID:       capapi.AttachmentIDPrefix + strings.Repeat(string(rune('a'+i)), 64),
			Filename: input.Filename, MediaType: input.MediaType, Size: 5,
		}.WithPreparedContent(capapi.AttachmentContent{Bytes: []byte("image")})
	}
	return out, nil
}

var _ = Describe("chat attachment resolution", func() {
	It("leaves assistant file outputs outside user-input resolution", func() {
		server := NewServer(Options{})
		req := ChatRequest{Messages: []UIMessage{{Role: "assistant", Parts: []UIPart{{
			Type: "file", MediaType: "image/png", URL: "https://example.com/result.png",
		}}}}}
		model := Model{ID: "openai/vision", Backend: capapi.BackendOpenAI, InputMediaTypes: []string{"image/*"}}

		Expect(server.resolveRequestAttachments(context.Background(), &req, model)).To(Succeed())
		Expect(req.Messages[0].Parts[0].URL).To(Equal("https://example.com/result.png"))
	})

	It("migrates legacy data URLs to durable descriptors before persistence", func() {
		server := NewServer(Options{AttachmentResolver: fakeAttachmentResolver{}})
		req := ChatRequest{Messages: []UIMessage{{Role: "user", Parts: []UIPart{
			{Type: "file", MediaType: "image/png", URL: "data:image/png;base64,aW1hZ2U=", Filename: "diagram.png"},
		}}}}
		model := Model{ID: "openai/vision", Backend: capapi.BackendOpenAI, InputMediaTypes: []string{"image/*"}}

		Expect(server.resolveRequestAttachments(context.Background(), &req, model)).To(Succeed())
		part := req.Messages[0].Parts[0]
		Expect(part.AttachmentID).To(HavePrefix(capapi.AttachmentIDPrefix))
		Expect(part.URL).To(Equal("/api/attachments/" + part.AttachmentID))
		encoded, err := json.Marshal(req.Messages[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(string(encoded)).NotTo(ContainSubstring("base64"))
	})

	It("carries prepared refs into file-only agent prompts", func() {
		server := NewServer(Options{AttachmentResolver: fakeAttachmentResolver{}})
		req := ChatRequest{Messages: []UIMessage{{Role: "user", Parts: []UIPart{{
			Type: "file", MediaType: "image/png", AttachmentID: capapi.AttachmentIDPrefix + strings.Repeat("a", 64),
		}}}}}
		model := Model{ID: "gpt-vision", Backend: capapi.BackendCodexAgent, InputMediaTypes: []string{"image/*"}}
		Expect(server.resolveRequestAttachments(context.Background(), &req, model)).To(Succeed())
		request := server.agentRequest(req, "")
		Expect(request.Prompt.User).To(BeEmpty())
		Expect(request.Prompt.Attachments).To(HaveLen(1))
		Expect(request.Prompt.Attachments[0].IsPrepared()).To(BeTrue())
	})
})
