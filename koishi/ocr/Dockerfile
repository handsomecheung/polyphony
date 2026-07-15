FROM python:3.11-slim-bookworm

# Install system dependencies including ocrmypdf, tesseract-ocr, and language packs
RUN apt-get update && apt-get install -y --no-install-recommends \
    ocrmypdf \
    tesseract-ocr-jpn \
    tesseract-ocr-eng \
    && rm -rf /var/lib/apt/lists/*

# Install python dependencies
RUN pip install --no-cache-dir fastapi uvicorn pdfplumber python-multipart

# Copy application code
ADD code /code
WORKDIR /code

EXPOSE 8000
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
