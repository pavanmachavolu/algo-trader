FROM python:3.11-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy script
COPY price_service.py .

# Make script executable
RUN chmod +x price_service.py

EXPOSE 5000

CMD ["python", "price_service.py"]
