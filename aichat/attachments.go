package aichat

import (
	"context"
	"fmt"
	"strings"

	capapi "github.com/flanksource/captain/pkg/api"
)

type AttachmentInput struct {
	ID        string
	URL       string
	Filename  string
	MediaType string
}

type AttachmentResolver interface {
	Resolve(context.Context, []AttachmentInput) ([]capapi.AttachmentRef, error)
}

func (s *Server) resolveRequestAttachments(ctx context.Context, req *ChatRequest, model Model) error {
	type location struct{ message, part int }
	var inputs []AttachmentInput
	var locations []location
	for messageIndex := range req.Messages {
		if req.Messages[messageIndex].Role != "user" {
			continue
		}
		for partIndex := range req.Messages[messageIndex].Parts {
			part := req.Messages[messageIndex].Parts[partIndex]
			if part.Type != "file" {
				continue
			}
			inputs = append(inputs, AttachmentInput{
				ID: part.AttachmentID, URL: part.URL,
				Filename: part.Filename, MediaType: part.MediaType,
			})
			locations = append(locations, location{message: messageIndex, part: partIndex})
		}
	}
	if len(inputs) == 0 {
		return nil
	}
	if s.opts.AttachmentResolver == nil {
		return fmt.Errorf("chat attachments require an AttachmentResolver")
	}
	resolved, err := s.opts.AttachmentResolver.Resolve(ctx, inputs)
	if err != nil {
		return err
	}
	if len(resolved) != len(inputs) {
		return fmt.Errorf("attachment resolver returned %d results for %d inputs", len(resolved), len(inputs))
	}
	for i, ref := range resolved {
		if !chatMediaTypeAccepted(model.InputMediaTypes, ref.MediaType) {
			return fmt.Errorf("model %s does not accept %s attachments", model.ID, ref.MediaType)
		}
		loc := locations[i]
		part := &req.Messages[loc.message].Parts[loc.part]
		part.AttachmentID = ref.ID
		part.URL = "/api/attachments/" + ref.ID
		part.Filename = ref.Filename
		part.MediaType = ref.MediaType
		part.resolvedAttachment = &resolved[i]
	}
	return nil
}

func chatMediaTypeAccepted(patterns []string, mediaType string) bool {
	for _, pattern := range patterns {
		if pattern == mediaType || strings.HasSuffix(pattern, "/*") && strings.HasPrefix(mediaType, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}
	return false
}
