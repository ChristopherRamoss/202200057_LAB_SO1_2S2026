Terminal 1:     En la ruta del /daemon

sudo ./proyecto2daemon


# Vuelta 1
- terminal 2

lsmod | grep continfo
cat /proc/continfo_pr2_so1_202200057 | head -20

# Vuelta 2
docker ps --format "table {{.ID}}\t{{.Names}}\t{{.Status}}"

sudo crontab -l 
docker ps

# Vuelta 3

ps aux | grep proyecto2daemon
valkey-cli -h localhost -p 6379 KEYS '*'
valkey-cli -h localhost -p 6379 GET <alguna_key_de_ejemplo>

<Estas_Son_key_de_ejemplo>: 
removed:total_count
system:ram:latest

# Vuelta 4

Podria spamear para ver la creacion y eliminacion EN OTRA TERMINAL

watch -n 1 "docker ps --filter label=proyecto2=true --format '{{.Image}}' | sed -e 's#.*go-client.*#ALTO#' -e 's#.*alpine.*#BAJO#' -e 's#.*intruder.*#INTRUSO#' | sort | uniq -c"

