docker build -t user:1.0 . -f Dockerfile_user
docker build -t gate:1.0 . -f Dockerfile_gate
docker build -t connector:1.0 . -f Dockerfile_connector
docker build -t hall:1.0 . -f Dockerfile_hall
docker build -t game:1.0 . -f Dockerfile_game