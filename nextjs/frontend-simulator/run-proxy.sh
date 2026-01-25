#!/bin/bash
echo "🚀 Starting Frontend Simulator (Next.js) for Proxy..."
echo "📍 Direct URL: http://localhost:3000/proxy/agent-1/"
echo "📍 Via Proxy: http://localhost:8080/proxy/agent-1/"
echo ""
BASE_PATH=/proxy/agent-1 pnpm run dev
