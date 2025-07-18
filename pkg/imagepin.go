package pkg

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sirupsen/logrus"
)

type ImagePinner struct {
	logger *logrus.Logger
	client *http.Client
}

func NewImagePinner(logger *logrus.Logger) *ImagePinner {
	return &ImagePinner{
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (ip *ImagePinner) PinImageToDigest(imageRef string) (string, error) {
	ip.logger.Debugf("Resolving image reference: %s", imageRef)

	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return "", fmt.Errorf("failed to parse image reference %s: %w", imageRef, err)
	}

	if strings.Contains(ref.String(), "@sha256:") {
		ip.logger.Debugf("Image %s already pinned to digest", imageRef)
		return imageRef, nil
	}

	var options []remote.Option
	options = append(options, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	
	if strings.Contains(ref.Context().RegistryStr(), "quay.io") {
		ip.logger.Debugf("Using anonymous auth for quay.io registry")
		options = []remote.Option{remote.WithAuth(authn.Anonymous)}
	}
	
	options = append(options, remote.WithContext(context.Background()))

	img, err := remote.Image(ref, options...)
	if err != nil {
		return "", fmt.Errorf("failed to get image manifest for %s: %w", imageRef, err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("failed to get digest for %s: %w", imageRef, err)
	}

	pinnedRef := fmt.Sprintf("%s@%s", ref.Context().String(), digest.String())
	ip.logger.Infof("Pinned %s to %s", imageRef, pinnedRef)
	
	return pinnedRef, nil
}

func (ip *ImagePinner) ProcessManifestContent(content []byte) ([]byte, error) {
	contentStr := string(content)
	
	imageRegex := regexp.MustCompile(`image:\s*['"]?([^'"\s]+)['"]?`)
	
	matches := imageRegex.FindAllStringSubmatch(contentStr, -1)
	if len(matches) == 0 {
		return content, nil
	}

	processedContent := contentStr
	pinnedImages := make(map[string]string)

	for _, match := range matches {
		originalImage := match[1]
		
		if strings.Contains(originalImage, "@sha256:") {
			continue
		}

		if pinnedRef, exists := pinnedImages[originalImage]; exists {
			processedContent = strings.ReplaceAll(processedContent, 
				fmt.Sprintf("image: %s", originalImage), 
				fmt.Sprintf("image: %s", pinnedRef))
			continue
		}

		pinnedRef, err := ip.PinImageToDigest(originalImage)
		if err != nil {
			ip.logger.Warnf("Failed to pin image %s: %v", originalImage, err)
			continue
		}

		pinnedImages[originalImage] = pinnedRef
		processedContent = strings.ReplaceAll(processedContent, 
			fmt.Sprintf("image: %s", originalImage), 
			fmt.Sprintf("image: %s", pinnedRef))
	}

	if len(pinnedImages) > 0 {
		ip.logger.Infof("Pinned %d images in manifest", len(pinnedImages))
	}

	return []byte(processedContent), nil
}

func (ip *ImagePinner) ProcessManifestFile(filePath string, content []byte) ([]byte, error) {
	ip.logger.Debugf("Processing manifest file: %s", filePath)
	return ip.ProcessManifestContent(content)
}


func (ip *ImagePinner) ProcessYAMLManifest(content []byte) ([]byte, error) {
	documents := strings.Split(string(content), "---")
	var processedDocs []string

	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		processedDoc, err := ip.ProcessManifestContent([]byte(doc))
		if err != nil {
			ip.logger.Warnf("Failed to process YAML document: %v", err)
			processedDocs = append(processedDocs, doc)
			continue
		}

		processedDocs = append(processedDocs, string(processedDoc))
	}

	return []byte(strings.Join(processedDocs, "\n---\n")), nil
}