FROM python:3.11-slim

# Install system dependencies including CA certificates
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    sudo \
    systemd \
    && update-ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create RKE2 directory structure that playbooks expect
RUN mkdir -p /etc/rancher/rke2/auth \
    /var/lib/rancher/rke2/server/tls \
    /var/lib/rancher/rke2/bin

# Create mock kubectl command
RUN echo '#!/bin/bash\necho "mocked kubectl"' > /var/lib/rancher/rke2/bin/kubectl && \
    chmod +x /var/lib/rancher/rke2/bin/kubectl

# Set working directory
WORKDIR /workspace

# Copy test requirements first (for layer caching)
COPY tests/ansible/requirements.txt /tmp/requirements.txt
RUN pip install --no-cache-dir --trusted-host pypi.org --trusted-host pypi.python.org --trusted-host files.pythonhosted.org -r /tmp/requirements.txt

# Copy the entire project
COPY . /workspace/

# Run tests
WORKDIR /workspace/tests/ansible
CMD ["pytest", "unit/", "-v", "--tb=short"]
