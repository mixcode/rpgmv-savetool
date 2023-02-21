
# A tool to manage rpgmaker-mv/mz save files  RPGツクールMV・MZのセーブデータをバックアップするツール

RPG-Maker MV (or MZ) save files are managed with a separated index file, 'global.rpgsave'.
This tool copies, moves, and deletes save files with proper handling of the index file.

## install
First you need a go language installation. Then type the commandline below.
```
go install github.com/mixcode/rpgmv-savetool@latest
```

## basic usage

Assume we are in the save file directory, usually at './www/save/'.

* show info of save files
```
rpgmv-savetool ls
```


* copy save 1 to a backup file
```
rpgmv-savetool cp file1.rpgsave backup_01.rpgarch

# or, with a shortcut
rpgmv-savetool cp @1 backup_01.rpgarch
```

* copy a backup file to save position 10
```
rpgmv-savetool cp backup_01.rpgarch file10.rpgsave

# or, with a shortcut
rpgmv-savetool cp backup_01.rpgarch @10
```

* move save 1 to save 9
```
rpgmv-savetool mv file1.rpgsave file9.rpgsave

# or
rpgmv-savetool mv @1 @9
```

* remove save 10
```
rpgmv-savetool rm file10.rpgsave

#or
rpgmv-savetool rm @10
```

## advanced

You could designate another directory as a source or a target of savefiles.
* copy all save files to another directory
```
rpgmv-savetool cp ./ ../backup_directory/

```
Or, you can save multiple save files to an archive file.
* copy all save files to a single archive file
```
# use -k to keep gaps between save slot numbers
rpgmv-savetool cp -k ./ backup_all.rpgarch

# show contents of the backup file
rpgmv-savetool ls backup_all.rpgarch
```

* copy save 1, 3, 5 to a backup file's save slot 11, 12, ...
```
rpgmv-savetool cp @1,3,5 backup_02.rpgarch@11-
```

* copy all save slots in a backup file to save 10, 11, 12, ...
```
rpgmv-savetool cp backup_02.rpgarch @10-
```

* move savefile 1 to 5 to 11 to 15
```
rpgmv-savetool mv -k @1-5 @11-
```

* remove all savefiles larger than 19
```
rpgmv-savetool rm @20-
```

## TODO
* 日本語ローカリゼーション
* -hで詳細の説明
