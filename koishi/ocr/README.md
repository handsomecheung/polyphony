# OCR

A lightweight PDF OCR microservice built with FastAPI and `ocrmypdf`. It is designed to act as a low-level base service that abstracts the complex system dependencies required for OCR (such as `tesseract-ocr`, ghostscript, and language packs), allowing client applications/skills to perform OCR via a simple HTTP API.

## API Specification

### GET `/ping`

Health check endpoint.

#### Response

- **Content-Type**: `application/json`
- **Body**:
  ```json
  {
    "ping": "pong"
  }
  ```

### POST `/ocr`

Performs OCR on a PDF document and returns the extracted text and the PDF (OCR-processed or original).

It automatically detects if the PDF already contains digital text (using `pdfplumber`). If text is found, it skips the heavy OCR process and returns the original text and PDF, unless forced.

#### Request

- **Content-Type**: `multipart/form-data`
- **Body parameters**:
  - `file`: `UploadFile` (Required) - The PDF file to be processed.
  - `lang`: `str` (Optional, default `"jpn+eng"`) - OCR language (e.g., `"jpn+eng"`, `"eng"`).
  - `force_ocr`: `bool` (Optional, default `false`) - Force OCR on all pages even if text is detected.
  - `redo_ocr`: `bool` (Optional, default `false`) - Redo OCR on pages that already contain text.
  - `skip_text`: `bool` (Optional, default `false`) - Skip OCR on pages that already contain text.

#### Response

- **Content-Type**: `application/json`
- **Body**:
  ```json
  {
    "text_base64": "Base64 encoded extracted text content from the PDF",
    "pdf_base64": "Base64 encoded content of the PDF (original or OCR-ed)",
    "ocr_performed": true
  }
  ```

---

## Usage Examples

### 1. Using curl
```bash
curl -X POST \
  -F "file=@/path/to/document.pdf" \
  -F "skip_text=true" \
  http://ocr-service/ocr > response.json
```

### 2. Python Integration
```python
import requests
import base64

def run_ocr(pdf_path, output_path=None):
    url = "http://ocr-service/ocr"
    
    with open(pdf_path, "rb") as f:
        files = {"file": (pdf_path, f, "application/pdf")}
        response = requests.post(url, files=files, data={"skip_text": "true"})
        
    if response.status_code != 200:
        raise Exception(f"OCR failed: {response.text}")
        
    result = response.json()
    extracted_text = result["text"]
    
    if output_path:
        pdf_bytes = base64.b64decode(result["pdf_base64"])
        with open(output_path, "wb") as f_out:
            f_out.write(pdf_bytes)
            
    return extracted_text
```

---

## Development & Deployment

### Build the Image
To build the Docker image in-cluster via Kaniko, run the build script:
```bash
./build.sh
```

### Deploy to Kubernetes
To deploy the service, apply the Kubernetes manifest using the deployment script:
```bash
./deploy.sh
```
This deploys the service in the `default` namespace under the `ocr-service` deployment. It uses **Sablier** to scale the workload down to `0` replicas when idle, waking it up on-demand when HTTP requests arrive at `ocr.<domain>`.
