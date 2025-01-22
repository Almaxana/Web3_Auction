### Аукцион по продаже билета (NFT-токена)

Тема проекта: *создание симуляции аукциона по продаже билета*.

Описание:

Объявляется аукцион на один билет (startAuction). Пользователи со своими кошельками (wallet.json) могут принять участие в аукционе. Для этого они используют клиентскую часть нашего приложения, где при помощи makeBet можно сделать ставку. Максимальная ставка из предложенных добавляется в хранилище ставок, так же как и хеш кошелька, с которого была сделана эта ставка. Хеш кошелька будет использован для определения текущего и конечного победителя в аукционе. По истечении определенного времени организатор заканчивает аукцион (finishAuction). Победитель определяется при помощи getPotentialWinner после окончания ауциона.

Что мы разыгрываем? Мы разыгрываем ticket - NFTшку в json формате.

Структура приложения:

1. Auction - основной контракт. Кроме функций deploy и update содержит функции начала аукциона, просмотра текущей ставки, получения текущего победителя и "сделать ставку". 
2. backend часть - реализованы startAuction, makeBet, getPotentialWinner, finishAuction, getNFT. В них: proceedMainTx и validateNotaryRequest функции.
3. client часть - main, в котором main с кейсами вызова, claimNotaryDeposit, makeNotaryRequest для каждого инструмента из backend.
4. nft - (тут дописать как и что с nft делаем)

________________________________________________________________________________________________

*Как запускается проект*

Создание хранилища:

1. Клонируем репозиторий https://git.frostfs.info/TrueCloudLab/frostfs-aio
2. В корне репозитория создаем docker контейнер 

$ make up

$ make refill - кладем "деньги"

3. В любой директории frostfs-aio/(например, s3-gw) создаем user-cli-cfg.yaml файл (название можно менять) со следующим содержимым: 

"wallet: /config/user-wallet.json

password: ""

rpc-endpoint: localhost:8080"

4. В директории frostfs-aio/s3-gw:

$ make cred

$ docker cp user-cli-cfg.yaml aio:/config/user-cli-cfg.yaml

$ docker exec aio frostfs-cli container create -c /config/user-cli-cfg.yaml --policy 'REP 1' --await

CID: 2aK1oHPovFPoqQwf6A3DPjwPBYUKmEqhWE81omGauNTH
awaiting...
container has been persisted on sidechain

$ docker exec aio frostfs-cli -c /config/user-cli-cfg.yaml ape-manager add --chain-id nyan --rule 'Allow Object.* *' --target-type container --target-name 2aK1oHPovFPoqQwf6A3DPjwPBYUKmEqhWE81omGauNTH (указываем полученный CID)

Полученный CID кладем в backend/config.yaml в storage_container.

Работа с контрактами:

1. Деплоим nft контракт:

$ neo-go contract compile -i nft/contract.go -o nft/contract.nef -m nft/contract.manifest.json -c nft/contract.yml

$ neo-go contract deploy -i nft/contract.nef -m nft/contract.manifest.json -r http://localhost:30333 -w ../../frostfs-aio/wallets/wallet1.json [ NhCHDEtGgSph1v6PmjFC1gtzJWNKtNSadk ]

2. В backend/config.yml и client/config.yml (и всех остальных конфигах клиента) указываем hash этого контракта.
Пишем:

$ neo-go util convert <hash этого контракта>

берем LE ScriptHash to Address и записываем в nftContractHashString контракта auction.

3. Деплоим auction контракт:

$ neo-go contract compile --in contract.go --out contract.nef -c contract.yml -m contract.manifest.json

$ neo-go contract deploy --in contract.nef --manifest contract.manifest.json --await -r http://localhost:30333 -w ../../../frostfs-aio/wallets/wallet1.json

Если надо обновить, то снова компилируем и вызываем update:

$ neo-go contract invokefunction -r http://localhost:30333 -w ../../../frostfs-aio/wallets/wallet1.json 45c904b50922ded714019a49796dafbdd981247f update filebytes:contract.nef filebytes:contract.manifest.json [ ]

В backend/config.yml и client/config.yml (и всех остальных конфигах клиента) указываем hash этого контракта.

4. Запускаем backend:

$ go run ./backend backend/config.yml

5. Создаем кошелек: 

$ neo-go wallet init -a -w wallet.json

6. 

