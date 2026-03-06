FROM python:3.12-slim

WORKDIR /app

COPY code/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY code/main.py .

CMD ["python", "-u", "main.py"]
