package initdetect

// Dockerfile templates — kept in sync with the server-side templates in
// apps/api/app/infrastructure/build/dockerfile_generator.py

const nodeSPATemplate = `FROM node:20-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=build /app/{{.BuildDir}} /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`

const nodeServerTemplate = `FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev
COPY . .
EXPOSE {{.Port}}
CMD ["npm", "start"]
`

const pythonTemplate = `FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
EXPOSE {{.Port}}
CMD ["python", "-m", "gunicorn", "--bind", "0.0.0.0:{{.Port}}", "app:app"]
`

const staticSiteTemplate = `FROM nginx:alpine
COPY . /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`
