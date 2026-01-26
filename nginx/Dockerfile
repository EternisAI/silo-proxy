FROM nginx:alpine

COPY nginx/silo-router.conf /etc/nginx/templates/default.conf.template

RUN rm -f /etc/nginx/conf.d/default.conf

EXPOSE ${NGINX_PORT:-8080}

CMD ["nginx", "-g", "daemon off;"]
