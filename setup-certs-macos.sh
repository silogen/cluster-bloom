#!/bin/bash

echo "Setting up certificates for macOS..."

# Export all macOS certificates
echo "Exporting macOS certificate bundle..."
security find-certificate -a -p /System/Library/Keychains/SystemRootCertificates.keychain > macos-ca-bundle.pem
security find-certificate -a -p /Library/Keychains/System.keychain >> macos-ca-bundle.pem

# Find enterprise certificates (adjust the search term as needed)
echo "Looking for enterprise certificates..."
security find-certificate -a -p | grep -A 50 -B 5 "YOUR_COMPANY" > enterprise-ca.pem 2>/dev/null || echo "No specific enterprise cert found, using full bundle"

# If no specific enterprise cert found, use the full bundle
if [ ! -s enterprise-ca.pem ]; then
    cp macos-ca-bundle.pem enterprise-ca.pem
fi

echo "Certificate bundle created: enterprise-ca.pem"

# Build Docker image with certificates
echo "Building Docker image with macOS certificates..."
docker build -f Dockerfile.macos -t bloom-test-macos .

# Run tests
echo "Running tests with enterprise certificates..."
docker run --rm -v $(pwd):/app -w /app bloom-test-macos

echo "Done!"