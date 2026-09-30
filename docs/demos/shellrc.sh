# Sourced, hidden, at the start of every tape: the demo HOME, wtm's shell
# integration (so `wtm go` can cd) and a short prompt.
export HOME=/tmp/wtm-demo/home PATH=/tmp/wtm-demo/bin:$PATH
eval "$(wtm shell-init)"
PS1='%F{blue}%1~%f %F{magenta}❯%f '
cd /tmp/wtm-demo/acme
clear
