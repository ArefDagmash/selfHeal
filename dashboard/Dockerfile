# ---- React dashboard (multi-stage, served by nginx) ----
FROM node:22-slim AS build
WORKDIR /app
COPY dashboard/package.json dashboard/package-lock.json ./
RUN npm install
COPY dashboard/ ./
RUN npm run build

FROM nginx:alpine
COPY dashboard/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 5173
CMD ["nginx", "-g", "daemon off;"]
