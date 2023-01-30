git config --global user.name $GIT_NAME
git config --global user.email $GIT_EMAIL
echo "$GIT_CREDENTIALS" > ~/.git-credentials
git config --global credential.helper "store --file ~/.git-credentials"
app
