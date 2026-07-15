import os
import tempfile
import subprocess
import base64
import logging
from fastapi import FastAPI, UploadFile, File, Form, HTTPException
import pdfplumber

class EndpointFilter(logging.Filter):
    def filter(self, record: logging.LogRecord) -> bool:
        return record.getMessage().find("/ping") == -1

logging.getLogger("uvicorn.access").addFilter(EndpointFilter())

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("ocr")

app = FastAPI(title="OCR API")

@app.get("/ping")
async def ping():
    return {"ping": "pong"}

@app.post("/ocr")
def perform_ocr(
    file: UploadFile = File(...),
    lang: str = Form("jpn+eng"),
    force_ocr: bool = Form(False),
    redo_ocr: bool = Form(False)
):
    if not file.filename.lower().endswith(".pdf"):
        raise HTTPException(status_code=400, detail="Only PDF files are supported.")

    with tempfile.TemporaryDirectory() as tmpdir:
        input_path = os.path.join(tmpdir, "input.pdf")
        output_path = os.path.join(tmpdir, "output.pdf")
        sidecar_path = os.path.join(tmpdir, "output.txt")

        # Save uploaded file
        try:
            contents = file.file.read()
            with open(input_path, "wb") as f:
                f.write(contents)
        except Exception as e:
            logger.error(f"Failed to save uploaded file: {e}")
            raise HTTPException(status_code=500, detail=f"Failed to save upload: {str(e)}")

        # Check if the PDF already contains text using pdfplumber
        has_text = False
        digital_text = ""
        try:
            with pdfplumber.open(input_path) as pdf:
                for page in pdf.pages:
                    page_text = page.extract_text()
                    if page_text:
                        digital_text += page_text + "\n"
            if digital_text.strip():
                has_text = True
        except Exception as e:
            logger.error(f"Failed to extract text with pdfplumber: {e}")

        # Decide whether to perform OCR
        run_ocr = force_ocr or redo_ocr or not has_text

        if not run_ocr:
            logger.info("PDF already contains text. Skipping OCR.")
            text_base64 = base64.b64encode(digital_text.encode("utf-8")).decode("utf-8")

            return {
                "text_base64": text_base64,
                "ocr_performed": False
            }

        # Construct ocrmypdf command with --sidecar
        cmd = ["ocrmypdf", "-l", lang, "--sidecar", sidecar_path]
        if force_ocr:
            cmd.append("--force-ocr")
        if redo_ocr:
            cmd.append("--redo-ocr")

        cmd.extend([input_path, output_path])

        logger.info(f"Running command: {' '.join(cmd)}")
        try:
            result = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, check=True)
            logger.info("OCR completed successfully")
        except subprocess.CalledProcessError as e:
            logger.error(f"ocrmypdf failed with code {e.returncode}")
            logger.error(f"stdout: {e.stdout}")
            logger.error(f"stderr: {e.stderr}")
            raise HTTPException(
                status_code=500, 
                detail=f"OCR execution failed: {e.stderr or e.stdout}"
            )

        # Read text from sidecar
        extracted_text = ""
        if os.path.exists(sidecar_path):
            try:
                with open(sidecar_path, "r", encoding="utf-8") as f:
                    extracted_text = f.read()
            except Exception as e:
                logger.error(f"Failed to read sidecar text file: {e}")

        text_base64 = base64.b64encode(extracted_text.encode("utf-8")).decode("utf-8")

        return {
            "text_base64": text_base64,
            "ocr_performed": True
        }

