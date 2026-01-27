package dto

type UploadResponse struct {
	Files []string `json:"files"`
}

type DeleteResponse struct {
	Message string `json:"message"`
}
