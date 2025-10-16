#!/bin/bash

# Certificate generation script for EasyAnyLink
# This script generates a CA and certificates for server, client, and gateway

set -e

CERTS_DIR="./certs"
DAYS_VALID=3650  # 10 years

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}EasyAnyLink Certificate Generator${NC}"
echo "===================================="
echo ""

# Create certs directory
mkdir -p "$CERTS_DIR"

# Generate CA private key
echo -e "${YELLOW}Generating CA private key...${NC}"
openssl genrsa -out "$CERTS_DIR/ca.key" 4096

# Generate CA certificate
echo -e "${YELLOW}Generating CA certificate...${NC}"
openssl req -new -x509 -days $DAYS_VALID -key "$CERTS_DIR/ca.key" \
    -out "$CERTS_DIR/ca.crt" \
    -subj "/C=US/ST=State/L=City/O=EasyAnyLink/OU=CA/CN=EasyAnyLink CA"

echo -e "${GREEN}✓ CA certificate generated${NC}"
echo ""

# Function to generate certificate
generate_cert() {
    local name=$1
    local cn=$2
    local san=$3
    
    echo -e "${YELLOW}Generating $name private key...${NC}"
    openssl genrsa -out "$CERTS_DIR/${name}.key" 2048
    
    echo -e "${YELLOW}Generating $name CSR...${NC}"
    openssl req -new -key "$CERTS_DIR/${name}.key" \
        -out "$CERTS_DIR/${name}.csr" \
        -subj "/C=US/ST=State/L=City/O=EasyAnyLink/OU=Agents/CN=${cn}"
    
    # Create extensions file if SAN is provided
    if [ -n "$san" ]; then
        cat > "$CERTS_DIR/${name}.ext" <<EOF
subjectAltName = $san
extendedKeyUsage = serverAuth, clientAuth
EOF
        echo -e "${YELLOW}Signing $name certificate with SAN...${NC}"
        openssl x509 -req -in "$CERTS_DIR/${name}.csr" \
            -CA "$CERTS_DIR/ca.crt" -CAkey "$CERTS_DIR/ca.key" \
            -CAcreateserial -out "$CERTS_DIR/${name}.crt" \
            -days $DAYS_VALID -extfile "$CERTS_DIR/${name}.ext"
        rm "$CERTS_DIR/${name}.ext"
    else
        echo -e "${YELLOW}Signing $name certificate...${NC}"
        openssl x509 -req -in "$CERTS_DIR/${name}.csr" \
            -CA "$CERTS_DIR/ca.crt" -CAkey "$CERTS_DIR/ca.key" \
            -CAcreateserial -out "$CERTS_DIR/${name}.crt" \
            -days $DAYS_VALID \
            -extensions v3_req \
            -extfile <(cat <<EOF
[v3_req]
extendedKeyUsage = serverAuth, clientAuth
EOF
)
    fi
    
    rm "$CERTS_DIR/${name}.csr"
    echo -e "${GREEN}✓ $name certificate generated${NC}"
    echo ""
}

# Generate server certificate
generate_cert "server" "easyanylink-server" "DNS:localhost,DNS:easyanylink-server,IP:127.0.0.1"

# Generate client certificate
generate_cert "client" "easyanylink-client"

# Generate gateway certificate
generate_cert "gateway" "easyanylink-gateway"

# Set permissions
chmod 600 "$CERTS_DIR"/*.key
chmod 644 "$CERTS_DIR"/*.crt

echo -e "${GREEN}All certificates generated successfully!${NC}"
echo ""
echo "Certificates location: $CERTS_DIR"
echo ""
echo "Files generated:"
echo "  - ca.crt, ca.key          (Certificate Authority)"
echo "  - server.crt, server.key  (Server certificate)"
echo "  - client.crt, client.key  (Client certificate)"
echo "  - gateway.crt, gateway.key (Gateway certificate)"
echo ""
echo -e "${YELLOW}IMPORTANT:${NC} Keep the private keys (.key files) secure!"
echo -e "${YELLOW}NOTE:${NC} For production, use proper hostnames/IPs in the SAN field"
