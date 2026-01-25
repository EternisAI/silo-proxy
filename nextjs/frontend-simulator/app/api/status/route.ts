import { NextResponse } from 'next/server';

export async function GET() {
  return NextResponse.json({
    status: 'running',
    service: 'frontend-simulator',
    timestamp: new Date().toISOString(),
    message: 'Connected via silo-proxy!',
  });
}
