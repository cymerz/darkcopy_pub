# Dockerfile for Next.js frontend
FROM node:20-alpine

WORKDIR /app

# Copy package management files
COPY package.json package-lock.json ./

# Install dependencies (including devDependencies needed for build)
RUN npm ci

# Copy the rest of the application
COPY . .

# Build the Next.js application
RUN npm run build

# Expose Next.js port
EXPOSE 3000

# Set environment to production
ENV NODE_ENV=production
ENV PORT=3000

# Start Next.js production server
CMD ["npm", "run", "start"]
