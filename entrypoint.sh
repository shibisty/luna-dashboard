#!/bin/sh
set -eu

echo "Installing dependencies..."
npm install

echo "Preparing..."
npx nuxi prepare

echo "Creating init.sql..."
npx prisma migrate diff --from-empty --to-schema-datamodel prisma/schema.prisma --script > init.sql

echo "Creating migrations (if schema changed)..."
npx prisma migrate deploy

echo "Generating Prisma client..."
npx prisma generate

echo "Building"
exec npm run build

echo "Starting Nuxt development server..."
exec npm run prod
