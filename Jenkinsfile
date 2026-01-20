@Library('jenkins.shared.library') _

pipeline {
  agent {
    label 'ubuntu_docker_label'
  }
  tools {
    go "Go 1.20"
  }
    options {
        checkoutToSubdirectory('src/github.com/infobloxopen/dapr')
  }
  environment {
    GOPATH = "$WORKSPACE"
    DIRECTORY = "src/github.com/infobloxopen/dapr"
    DOCKER_IMAGE = "infoblox/dapr"
    
  }
  stages {
    stage("Setup") {
      steps {
        prepareBuild()
        withCredentials([string(credentialsId: 'GITHUB_TOKEN', variable: 'GITHUB_PAT')]) {
          dir("$DIRECTORY") {
            sh 'git config --global url."https://\$GITHUB_PAT:x-oauth-basic@github.com/".insteadOf "https://github.com/"'
          }
        }
      }
    }
   stage("Test") {
      steps {
        sh "cd $DIRECTORY && make test"
      }
    }
    stage("build-and-archive-binaries-linux-amd64"){
         steps {
          sh "cd $DIRECTORY && make tidy && make release GOOS='linux' GOARCH='amd64' "
        }
      }
    stage("Build-And-Push-Docker") {
       steps {
        dir ("$DIRECTORY") {
          withDockerRegistry([credentialsId: "dockerhub-bloxcicd", url: ""]) {
            sh "make docker-push-harbor GOOS='linux' GOARCH='amd64'"
          }
        }
      }
    }
    stage("Package-Helm-Chart") {
      steps {
        dir ("$DIRECTORY") {
          sh "make push-chart"
        }
        dir("${WORKSPACE}/${DIRECTORY}") {
          archiveArtifacts artifacts: 'charts/*.tgz'
          archiveArtifacts artifacts: 'charts/*.build.properties'
        }
      }
    }
  }
  post {
    success {
      dir("${WORKSPACE}/${DIRECTORY}"){
        finalizeBuild("", "charts/*.build.properties")
      }
    }
    cleanup {
      withCredentials([string(credentialsId: 'GITHUB_TOKEN', variable: 'GITHUB_PAT')]) {
        dir("$DIRECTORY") {
          sh "make clean || true"
          sh 'git config --global --unset url."https://$GITHUB_PAT:x-oauth-basic@github.com/".insteadOf'
        }
      }
      cleanWs()
    }
  }
}
