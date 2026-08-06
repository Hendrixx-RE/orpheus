package pm

import (
	"encoding/json"
	"strings"
)

type Npm struct{}

func NewNpm() *Npm { return &Npm{} }

func (p *Npm) Name() string { return "node" }

func (p *Npm) UninstallCmd(names []string) []string {
	args := []string{"npm", "uninstall", "-g"}
	return append(args, names...)
}

func (p *Npm) UninstallOrphansCmd() []string {
	return []string{"npm", "prune", "-g"}
}

func (p *Npm) GetOrphans() ([]string, error) {
	out, err := runCmdAllowExit1("npm", "prune", "-g", "--dry-run")
	if err != nil {
		return []string{}, nil
	}
	str := string(out)
	if strings.TrimSpace(str) == "" || strings.Contains(str, "up to date") {
		return []string{}, nil
	}
	lines := strings.Split(str, "\n")
	var orphans []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && (strings.HasPrefix(l, "removed") || strings.HasPrefix(l, "unneeded")) {
			orphans = append(orphans, l)
		}
	}
	return orphans, nil
}

func (p *Npm) ListAll() ([]Package, error) {
	script := `const fs=require('fs'),path=require('path'),{execSync}=require('child_process');
try{
let root=execSync('npm root -g').toString().trim();
let pkgs=[];
let getSize=d=>{let s=0;try{let c=fs.readdirSync(d);for(let f of c){let p=path.join(d,f),st=fs.statSync(p);if(st.isDirectory())s+=getSize(p);else s+=st.size;}}catch(e){}return s;};
for(let d of fs.readdirSync(root)){if(d.startsWith('.'))continue;try{let p=require(path.join(root,d,'package.json'));pkgs.push({name:p.name||d,version:p.version||'',size:getSize(path.join(root,d))});}catch(e){}}
console.log(JSON.stringify(pkgs));
}catch(e){console.log('[]');}`

	out, err := runCmd("node", "-e", script)
	if err != nil {
		return nil, err
	}

	var items []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Size    int64  `json:"size"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, err
	}

	var pkgs []Package
	for _, item := range items {
		if item.Name == "" {
			continue
		}
		pkgs = append(pkgs, Package{
			Name:          item.Name,
			Version:       item.Version,
			Size:          item.Size,
			InstallReason: "Explicitly installed",
		})
	}

	return pkgs, nil
}

func (p *Npm) GetPackage(name string) (*Package, error) {
	out, err := runCmd("npm", "view", name, "--json")
	if err != nil {
		return nil, err
	}

	var data struct {
		Name        string            `json:"name"`
		Version     string            `json:"version"`
		Description string            `json:"description"`
		Dependencies map[string]string `json:"dependencies"`
	}

	if err := json.Unmarshal(out, &data); err != nil {
		return nil, err
	}

	pkg := &Package{
		Name:          data.Name,
		Version:       data.Version,
		Description:   data.Description,
		InstallReason: "Explicitly installed",
	}

	for dep := range data.Dependencies {
		pkg.Dependencies = append(pkg.Dependencies, dep)
	}

	return pkg, nil
}
