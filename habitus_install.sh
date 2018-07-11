#!/bin/bash

version=1.0.3

# bash utilities credit: http://natelandau.com/bash-scripting-utilities/


#Formatting output
bold=$(tput bold)
reset=$(tput sgr0)
red=$(tput setaf 1)
green=$(tput setaf 76)
tan=$(tput setaf 3)
e_success() { printf "${bold}${green}%s${reset}\n" "$@"
}
e_error() { printf "${bold}${red}%s${reset}\n" "$@"
}
e_warning() { printf "${tan}%s${reset}\n" "$@"
}
e_info() { printf "${bold}%s${reset}\n" "$@"
}

#Check target
type_exists() {
if [ $(type -P $1) ]; then
  return 0
fi
return 1
}
is_os() {
  if [[ "${OSTYPE}" == $1* ]]; then
    return 0
  fi
  return 1
}
is_64() {
  if [ `uname -m` == 'x86_64' ]; then
    return 0 # 64-bit stuff here
  fi
  return 1 # 32-bit stuff here
}

do_install() {
  if is_os "darwin"; then
    e_info "Downloading Habitus for macOS..."
    `rm -f /tmp/habitus &> /dev/null`
    `curl -L --progress-bar -o /tmp/habitus https://github.com/cloud66-oss/habitus/releases/download/$version/habitus_darwin_amd64`
  elif is_os "linux"; then
    if [[ is_64 ]]; then
	  e_info "Downloading Habitus for Linux x64..."
	  `rm -f /tmp/habitus &> /dev/null`
	  `curl -L --progress-bar -o /tmp/habitus https://github.com/cloud66-oss/habitus/releases/download/$version/habitus_linux_amd64`
    else
	  e_info "Downloading Habitus for Linux x32..."
	  `rm -f /tmp/habitus &> /dev/null`
	  `curl -L --progress-bar -o /tmp/habitus https://github.com/cloud66-oss/habitus/releases/download/$version/habitus_linux_386`
    fi
  else
  	e_error "Aborted: Unable to detect your operating system and architecture!"
  	e_warning "Please download Habitus manually from: https://github.com/cloud66-oss/habitus/releases"
  	exit 1
  fi
  # extract the archive to local home
  printf "Copying Habitus to $USER_HOME/.habitus/habitus ...\n"
  `mkdir -p $USER_HOME/.habitus`
  `rm -f $USER_HOME/.habitus/habitus &> /dev/null`
  `cp /tmp/habitus  $USER_HOME/.habitus/habitus &> /dev/null`
  printf "Making habitus command executable ...\n"
  if [ $UID -eq 0 ] ; then
    `chown $SUDO_USER $USER_HOME/.habitus`
    `chown $SUDO_USER $USER_HOME/.habitus/habitus`
  fi
  `chmod +x $USER_HOME/.habitus/habitus`
  printf "Creating Habitus symlink in /usr/local/bin/habitus ...\n"
  `unlink /usr/local/bin/habitus &> /dev/null`
  `ln -nfs $USER_HOME/.habitus/habitus /usr/local/bin/habitus &> /dev/null`
  if [ $? -eq 0 ] ; then
  	e_info "The 'habitus' command should now be available"
  	e_success "Successfully installed Habitus! Go build some images!"
  else
	e_warning "Warning: Unable to create a symlink for Habitus"
	e_warning "Please create your symlink manually from $USER_HOME/.habitus/habitus"
  fi
}

e_success "Installing Habitus V$version"
# check if running as sudoer
if [ $UID -eq 0 ] ; then
	USER_HOME="/home/"$SUDO_USER
else
	USER_HOME=$HOME
fi
if type_exists 'tar'; then
  do_install
else
  e_error "Aborted: 'tar' is required to extract the binary. Please install 'tar' first"
  exit 1
fi
printf "\n"
