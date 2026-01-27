#!/bin/bash
curl -X POST https://silo-proxy-production.up.railway.app/list -H "Content-Type: application/json" -d '{"target_dir":"/cert"}'
