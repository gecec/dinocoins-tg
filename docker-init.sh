#!/bin/sh
echo "prepare environment"

if [ -d "/srv/var" ]; then
  echo "changing ownership of /srv/var to app:app (dinocoins-tg user inside the container)"
  chown -R app:app /srv/var || echo "WARNING: /srv/var ownership change failed, if application will fail that might be the reason"
else
  echo "ERROR: /srv/var doesn't exist, which means that state of the application"
  echo "ERROR: will be lost on container stop or restart."
  echo "ERROR: Please mount local directory to /srv/var in order for it to work."
  exit 199
fi