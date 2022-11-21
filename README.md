# GmailFetcher

Gmail API を使ってメールの添付ファイルをダウンロードするサンプルプログラムです。

# 事前準備１

github.com/mattn/go-sqlite3 はcgoパッケージであるため、
go-sqlite3を使用してアプリをビルドする場合は、gccが必要です。

Windowsで利用する場合は以下のアドレスからgcc toolchainのインストールを行います。
https://jmeubank.github.io/tdm-gcc/

以下のインストールファイルをダウンロードしてインストールします。
https://github.com/jmeubank/tdm-gcc/releases/download/v10.3.0-tdm64-2/tdm64-gcc-10.3.0-2.exe

インストールをしたらインストールフォルダのbinをPATHに追加します。
デフォルトでは
C:\TDM-GCC-64\bin
かもしれません。

# 事前準備２
https://developers.google.com/gmail/api/quickstart/go 
の通りですが記載します。

## プロジェクトを作成する

事前にプロジェクトを作成します。

## API を有効にする

以下のアドレスの画面から Gmail API を有効にします。
https://console.cloud.google.com/flows/enableapi?apiid=gmail.googleapis.com

![img.png](docs/img.png)

## OAuth同意画面

スコープでは、`.../auth/gmail.modify`  にチェックを付けて権限を付与します。

![img_1.png](docs/img_1.png)

## 認証情報ファイルを作成する

https://console.cloud.google.com/apis/credentials

左側のハンバーガーメニューから
APIとサービス ＞ 認証情報
をクリックします。

![img_7.png](docs/img_7.png)

認証情報を作成 をクリックして OAuth クライアントID をクリックする。

![img_8.png](docs/img_8.png)

デスクトップアプリで作成します。

![img_9.png](docs/img_9.png)

OAuthクライアントを作成しました。と表示されるので
JSONをダウンロードをクリックしてファイルを取得します。
ファイルは動作させる環境にコピーしておきますが、ファイル名を credentials.json としておく必要があります。

![img_3.png](docs/img_3.png)

## OAuth 認証を行う

```shell
git clone gmailfetcher
cd gmailfetcher
go mod tidy
go run fetcher.go
```

ここで以下のスクリーンショットのように oauth 認証へのアドレスが表示されるので
コンソールからアドレスをブラウザに貼り付けます。

![img_4.png](docs/img_6.png)

アカウントを選択し

![img_2.png](docs/img_2.png)

続行をクリックし

![img_3.png](docs/img_3.png)

localhostにリダイレクトしエラーになりますが、
ブラウザのアドレスバーに必要なパラメータがありますので
codeパラメータの値（～&code=xxxxx&～ となっている xxxxx の部分）をコピーし、
コンソールに貼り付けてエンターキーを押します。

# コマンドライン

go run fetcher.go 
 -after gmail の after フィルタを利用する
   1d: 本日から1日前からのメールを取得する
   2w: 本日から2週間前からのメールを取得する
   3m: 本日から3か月前からのメールを取得する
   4y: 本日から4年前からのメールを取得する

# Author, Contributor

Akifumi NAKAMURA (@tmpz84)

# LICENSE

MIT

