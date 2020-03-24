# Сервис для ресайза и конвертации картинок

### Сервис на вход принимает урл вида:

```bash
http://localhost/_nuxt/img/82a1fe2.png/resizer/x/y
```

1. x - ширина в пикселях
2. y - высота в пикселях
3. Для автомаштабирования ширины или высоты, передаете на их месте 0

### На выходе получаете файл формата jpeg заданного размера

## Для получения обработанных картинок, нужно внести правила в nginx.
### Правила для Nginx:

```bash
location ~ /resizer/ {
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_pass http://service:port;
}
```

## Запуск сервиса:
```bash
docker build -t go-image-resizer .
docker run -it -p 8080:8080 go-image-resizer:latest
```

## Настроки сервиса:
1. Урл для получения статуса - /api/system/version
2. Переменная среды PORT - порт на котором будет слушать приложение (по умолчанию: 8080)
3. Переменная среды SENTRYURL - урл на Sentry для логирования ошибок
4. Переменная среды BRANCH - версия сервиса
5. Переменная среды CACHEINSECONDS - время обновления кэша с картинками в секундах (по умолчанию: 3600 )
