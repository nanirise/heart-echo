# ===== stage 1: 构建前端 =====
FROM node:20-alpine AS build
WORKDIR /app
# npm ci 需要 package-lock.json —— 必须提交进仓库，否则部署时构建失败
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
# ⚠️ Vite 的环境变量在「构建时」注入，运行期改 .env 无效。
# 生产前后端同源，API 走 Nginx 的 /api 转发，所以是相对路径 /api/v1。
ARG VITE_API_BASE_URL=/api/v1
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN npm run build

# ===== stage 2: Nginx 托管静态资源 + 反向代理 =====
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
