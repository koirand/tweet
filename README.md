# tweet

![header-882x220](https://user-images.githubusercontent.com/17229643/74096683-3f986500-4b45-11ea-943d-48448a9999f5.png)

Simple command just to tweet.

## Install

```sh
$ go get github.com/koirand/tweet
```

## Usage

```sh
$ tweet
```

It opens text editor which is settled in $EDITOR (default vim).
When you have done writing, the text is posted to twitter.

And you can also use pipe.

```sh
$ cat foo.txt | tweet
```

At first time, authentication info is required.
It's saved in the following file.

#### Linux, Mac user
~/.config/koirand-tweet/settings.json

#### Windows user
%USERPROFILE%/Application Data/koirand-tweet/settings.json

