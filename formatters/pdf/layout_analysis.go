package pdf

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// LayoutAnalysisResult contains the results of analyzing a PDF layout
type LayoutAnalysisResult struct {
	PageWidth           int      `json:"page_width"`
	PageHeight          int      `json:"page_height"`
	LeftColumnBounds    *Bounds  `json:"left_column_bounds"`
	RightColumnBounds   *Bounds  `json:"right_column_bounds"`
	LeftColumnWidth     int      `json:"left_column_width"`
	RightColumnWidth    int      `json:"right_column_width"`
	LeftColumnRatio     float64  `json:"left_column_ratio"`
	RightColumnRatio    float64  `json:"right_column_ratio"`
	HasSideBySideLayout bool     `json:"has_side_by_side_layout"`
	LayoutValid         bool     `json:"layout_valid"`
	ValidationErrors    []string `json:"validation_errors"`
}

// Bounds represents a rectangular area
type Bounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ConvertPDFToPNG converts a PDF to PNG using Ghostscript or ImageMagick
func ConvertPDFToPNG(pdfData []byte, outputPath string, dpi int) error {
	if dpi <= 0 {
		dpi = 300 // Default DPI
	}

	// Create temporary PDF file
	tempPDF, err := os.CreateTemp("", "layout_analysis_*.pdf")
	if err != nil {
		return fmt.Errorf("failed to create temp PDF file: %w", err)
	}
	defer os.Remove(tempPDF.Name())

	// Write PDF data to temp file
	if _, err := tempPDF.Write(pdfData); err != nil {
		tempPDF.Close()
		return fmt.Errorf("failed to write PDF data: %w", err)
	}
	tempPDF.Close()

	// Try Ghostscript first
	if err := convertPDFToPNGWithGhostscript(tempPDF.Name(), outputPath, dpi); err == nil {
		return nil
	}

	// Fallback to ImageMagick
	if err := convertPDFToPNGWithImageMagick(tempPDF.Name(), outputPath, dpi); err == nil {
		return nil
	}

	// Try pdftoppm as a last resort
	if err := convertPDFToPNGWithPDFToPPM(tempPDF.Name(), outputPath, dpi); err == nil {
		return nil
	}

	return fmt.Errorf("no PDF to PNG converter available (tried ghostscript, imagemagick, pdftoppm)")
}

// convertPDFToPNGWithGhostscript uses Ghostscript to convert PDF to PNG
func convertPDFToPNGWithGhostscript(pdfPath, outputPath string, dpi int) error {
	cmd := exec.Command("gs",
		"-dNOPAUSE",
		"-dBATCH",
		"-dSAFER",
		"-sDEVICE=png16m",
		fmt.Sprintf("-r%d", dpi),
		"-dFirstPage=1",
		"-dLastPage=1",
		fmt.Sprintf("-sOutputFile=%s", outputPath),
		pdfPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ghostscript failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// convertPDFToPNGWithImageMagick uses ImageMagick to convert PDF to PNG
func convertPDFToPNGWithImageMagick(pdfPath, outputPath string, dpi int) error {
	cmd := exec.Command("convert",
		"-density", strconv.Itoa(dpi),
		"-quality", "100",
		fmt.Sprintf("%s[0]", pdfPath), // First page only
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("imagemagick failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// convertPDFToPNGWithPDFToPPM uses pdftoppm to convert PDF to PNG
func convertPDFToPNGWithPDFToPPM(pdfPath, outputPath string, dpi int) error {
	// pdftoppm outputs to PPM, so we need to convert to PNG
	tempPPM := strings.TrimSuffix(outputPath, ".png") + ".ppm"
	defer os.Remove(tempPPM)

	// Convert PDF to PPM
	cmd := exec.Command("pdftoppm",
		"-png",
		"-r", strconv.Itoa(dpi),
		"-f", "1", "-l", "1", // First page only
		pdfPath,
		strings.TrimSuffix(outputPath, ".png"),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pdftoppm failed: %w (output: %s)", err, string(output))
	}

	// pdftoppm with -png creates filename-1.png, so we need to rename
	generatedFile := strings.TrimSuffix(outputPath, ".png") + "-1.png"
	if _, err := os.Stat(generatedFile); err == nil {
		return os.Rename(generatedFile, outputPath)
	}

	return fmt.Errorf("pdftoppm did not generate expected output file")
}

// AnalyzePDFLayout converts a PDF to PNG and analyzes its layout structure
func AnalyzePDFLayout(pdfData []byte, targetLeftRatio, targetRightRatio float64) (*LayoutAnalysisResult, error) {
	// Create temporary PNG file
	tempPNG, err := os.CreateTemp("", "layout_analysis_*.png")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp PNG file: %w", err)
	}
	defer os.Remove(tempPNG.Name())
	tempPNG.Close()

	// Convert PDF to PNG
	if err := ConvertPDFToPNG(pdfData, tempPNG.Name(), 300); err != nil {
		return nil, fmt.Errorf("failed to convert PDF to PNG: %w", err)
	}

	// Analyze the PNG image
	return AnalyzeImageLayout(tempPNG.Name(), targetLeftRatio, targetRightRatio)
}

// AnalyzeImageLayout analyzes a PNG image to determine layout structure
func AnalyzeImageLayout(imagePath string, targetLeftRatio, targetRightRatio float64) (*LayoutAnalysisResult, error) {
	// Open and decode the PNG image
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	result := &LayoutAnalysisResult{
		PageWidth:        width,
		PageHeight:       height,
		ValidationErrors: []string{},
	}

	// Analyze column layout using content detection
	leftBounds, rightBounds, err := detectColumnBounds(img)
	if err != nil {
		result.ValidationErrors = append(result.ValidationErrors, fmt.Sprintf("Column detection failed: %v", err))
		return result, nil
	}

	if leftBounds != nil && rightBounds != nil {
		result.LeftColumnBounds = leftBounds
		result.RightColumnBounds = rightBounds
		result.LeftColumnWidth = leftBounds.Width
		result.RightColumnWidth = rightBounds.Width
		result.HasSideBySideLayout = true

		// Calculate ratios
		totalWidth := width
		result.LeftColumnRatio = float64(leftBounds.Width) / float64(totalWidth)
		result.RightColumnRatio = float64(rightBounds.Width) / float64(totalWidth)

		// Validate ratios against targets
		leftRatioError := abs(result.LeftColumnRatio - targetLeftRatio)
		rightRatioError := abs(result.RightColumnRatio - targetRightRatio)

		const tolerance = 0.15 // 15% tolerance for content-based analysis

		if leftRatioError > tolerance {
			result.ValidationErrors = append(result.ValidationErrors,
				fmt.Sprintf("Left column ratio %.3f differs from target %.3f by %.3f (exceeds tolerance %.3f)",
					result.LeftColumnRatio, targetLeftRatio, leftRatioError, tolerance))
		}

		if rightRatioError > tolerance {
			result.ValidationErrors = append(result.ValidationErrors,
				fmt.Sprintf("Right column ratio %.3f differs from target %.3f by %.3f (exceeds tolerance %.3f)",
					result.RightColumnRatio, targetRightRatio, rightRatioError, tolerance))
		}

		result.LayoutValid = len(result.ValidationErrors) == 0
	} else {
		result.ValidationErrors = append(result.ValidationErrors, "No side-by-side layout detected")
	}

	return result, nil
}

// detectColumnBounds detects column boundaries in an image using content analysis
func detectColumnBounds(img image.Image) (*Bounds, *Bounds, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Create a simple content density map
	// We'll scan horizontal strips to find content distribution
	contentMap := make([]int, width)

	// Sample vertical strips to detect content
	sampleHeight := height / 10 // Sample every 10th row
	if sampleHeight < 1 {
		sampleHeight = 1
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleHeight {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()

			// Convert to grayscale (0-65535 range)
			gray := (r + g + b) / 3

			// Content detection: anything that's not close to white
			const whiteThreshold = 60000 // Adjust this threshold as needed
			if gray < whiteThreshold {
				contentMap[x-bounds.Min.X]++
			}
		}
	}

	// Find content clusters to identify column boundaries
	// Look for a gap in the middle that separates two content areas
	leftContentEnd := -1
	rightContentStart := -1

	// Find the end of left content (scanning from left)
	for i := 0; i < width/2; i++ {
		if contentMap[i] > 0 {
			leftContentEnd = i
		}
	}

	// Find the start of right content (scanning from right)
	for i := width - 1; i >= width/2; i-- {
		if contentMap[i] > 0 {
			rightContentStart = i
			break
		}
	}

	if leftContentEnd == -1 || rightContentStart == -1 {
		return nil, nil, fmt.Errorf("could not detect column boundaries")
	}

	// Check if there's a reasonable gap between columns
	gap := rightContentStart - leftContentEnd
	if gap < width/20 { // Gap should be at least 5% of page width
		return nil, nil, fmt.Errorf("insufficient gap between columns")
	}

	// Define column bounds
	leftBounds := &Bounds{
		X:      0,
		Y:      0,
		Width:  leftContentEnd + gap/2,
		Height: height,
	}

	rightBounds := &Bounds{
		X:      rightContentStart - gap/2,
		Y:      0,
		Width:  width - (rightContentStart - gap/2),
		Height: height,
	}

	return leftBounds, rightBounds, nil
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// SaveAnalysisDebugImage saves a debug version of the image with column boundaries marked
func SaveAnalysisDebugImage(imagePath, outputPath string, result *LayoutAnalysisResult) error {
	// This is a simplified version - in a full implementation, you would
	// draw rectangles around the detected columns

	// For now, just copy the original image and add text annotations
	input, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer output.Close()

	img, err := png.Decode(input)
	if err != nil {
		return err
	}

	// In a complete implementation, you would draw overlay rectangles here
	// For now, just save the original image
	return png.Encode(output, img)
}
