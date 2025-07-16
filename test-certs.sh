#!/bin/bash

echo "Testing certificate access..."

# Test if we can access the problematic URLs
echo "Testing Go proxy access..."
curl -I https://proxy.golang.org/github.com/gdamore/tcell/v2/@v/v2.8.1.info

echo -e "\nTesting direct GitHub access..."
curl -I https://github.com/gdamore/tcell

echo -e "\nListing enterprise certificates..."
security find-certificate -a -c "Enterprise" -p | head -20

echo -e "\nDone!"