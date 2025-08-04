#!/bin/bash

# Download ONNX models for embeddings

MODEL_DIR="models"
mkdir -p "$MODEL_DIR"

echo "Downloading all-MiniLM-L6-v2 ONNX model..."

# Download from Hugging Face
MODEL_URL="https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"
TOKENIZER_URL="https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json"
CONFIG_URL="https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/config.json"

echo "Downloading model..."
curl -L "$MODEL_URL" -o "$MODEL_DIR/all-MiniLM-L6-v2.onnx"

echo "Downloading tokenizer..."
curl -L "$TOKENIZER_URL" -o "$MODEL_DIR/tokenizer.json"

echo "Downloading config..."
curl -L "$CONFIG_URL" -o "$MODEL_DIR/config.json"

echo "Model files downloaded to $MODEL_DIR/"
ls -lh "$MODEL_DIR/"