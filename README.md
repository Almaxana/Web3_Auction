# Инструкция по запуску
## Подготовка
Клонируем `frostfs-aio`, переходим на ветку `nightly-v1.7`. Если уже это делали и хотим все начать с чистого листа, то, чтобы удалить все работающие контейнеры вместе с задеплоенными контрактами пишем:
```bash
make down clean
```
Поднимаем `frostfs-aio`
```bash
make up
```

Кладем денег на `wallet1.json`, который будет платить здесь за всё:
 ```bash
make refill
```

Создаем контейнер - storage node
````bash
1)  make cred

2) в любой директории frostfs-aio создаем файл user-cli-cfg.yaml с содержимым:
wallet: /config/user-wallet.json
password: ""
rpc-endpoint: localhost:8080

3) docker cp user-cli-cfg.yaml aio:/config/user-cli-cfg.yaml
4) docker exec aio frostfs-cli -c /config/user-cli-cfg.yaml ape-manager add --chain-id nyan --rule 'Allow Object.* *' --target-type container --target-name <CID полученный предыдущей командой>
````
❗️в `backend/config.yaml` в `storage_container` указываем полученный CID

## nft
```
neo-go contract compile -i nft/contract.go -o nft/contract.nef -m nft/contract.manifest.json -c nft/contract.yml
neo-go contract deploy -i nft/contract.nef -m nft/contract.manifest.json -r http://localhost:30333 -w ../../frostfs-aio/wallets/wallet1.json [ NhCHDEtGgSph1v6PmjFC1gtzJWNKtNSadk ]
```
❗️В `backend/config.yml` и `client/config.yml` (и всех остальных конфигах клиента) указываем hash этого контракта.

Пишем
```
neo-go util convert <hash этого контракта>
```
и берем `LE ScriptHash to Address` и ❗️записываем в `nftContractHashString` контракта `auction`

## auction
```
neo-go contract compile --in contract.go --out contract.nef -c contract.yml -m contract.manifest.json
neo-go contract deploy --in contract.nef --manifest contract.manifest.json --await -r http://localhost:30333 -w ../../../frostfs-aio/wallets/wallet1.json
```
Если надо обновить, то снова компилируем контракт и вызываем у него update
```
neo-go contract invokefunction -r http://localhost:30333 -w ../../../frostfs-aio/wallets/wallet1.json 45c904b50922ded714019a49796dafbdd981247f update filebytes:contract.nef filebytes:contract.manifest.json [ ]
```
❗️В `backend/config.yml` и `client/config.yml` (и всех остальных конфигах клиента) указываем hash этого контракта

## backend

Запускаем backend

```bash
go run ./backend backend/config.yml
```

## client

Запускаем client

```bash
go run ./client client/config.yml
```

Он будет работать постоянно, так же как и backend. В терминале клиента нужно вводить команды. Примеры
```bash
getNFT
startAuction 	dce48fffd5f2b57c8c76c407e26da2a99dce8b59076fc7805c8e6326389c20fc 	300
exit
```

## extra commands
Посмотреть, свойства данного nft
```
curl http://localhost:5555/properties/6b86b273ff34fce19d6b804eff5a3f5747ada4eaa22f1d49c01e52ddb7875b4b | jq
```
Увидеть json, на который ссылается данный nft
```
http://localhost:8081/get/<address>
```

Можно вызвать непосредственно функции контракта auction из консоли (даны пары команд: первая для вызова функции контракта, вторая - для конвертации полученного ответа в человекочиатемый вид)
 - ShowLotId
```
neo-go contract testinvokefunction -r http://localhost:30333 	45c904b50922ded714019a49796dafbdd981247f showLotId
echo "nbWAJ75S0nDn7lc4XIcx2O68bG3rceLI6hHdxb1YgnM=" | base64 --decode | xxd -p
```

 - ShowCurrentBet
```
neo-go contract testinvokefunction -r http://localhost:30333 	5af416d1ec7825786474f26a92e3a8a772e22810 showCurrentBet
neo-go util convert 9AE=
```
