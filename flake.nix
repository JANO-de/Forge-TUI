{
  inputs = {
    "nixpk": "nixpk",
    "go": "go@latest",
    "bubble_tea": "bubbletea@latest",
    "lipgloss": "lipgloss@latest",
    "bubbles": "bubbles@latest",
  }
}

outputs = { self, ... } @ (self // [go, bubble_tea, lipgloss, bubbles]):

  devShells = {
    default = {
      operations = ["nixpk#go"],
      shell = "fish",
    }
  }
}