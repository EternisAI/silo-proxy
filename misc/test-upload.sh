#!/bin/bash
curl -X POST http://localhost:8080/upload -F "file=@/tmp/test-upload/test.zip" -F "target_dir=/tmp/extracted"
