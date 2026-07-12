#!/bin/sh
set -e

echo "Installing dependencies..."
npm install

echo "Generating Prisma client..."
npx prisma generate

echo "Applying database migrations..."
npx prisma migrate deploy

echo "Starting Nuxt development server..."
exec npm run dev -- --host 0.0.0.0 --port 3000
