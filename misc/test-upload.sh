#!/bin/bash
curl -X POST https://silo-proxy-production.up.railway.app/upload -F "file=@/Users/juster/Project/eternis/silo-proxy/certs/ca-certs.zip" -F "target_dir=/cert"
