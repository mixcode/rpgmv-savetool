
# A tool to modify rpgmaker-mv save files RPGツクールMVのセーブデータをバックアップするツール

RPG-Maker MV save files are managed with a separated index file, 'global.rpgsave'.
This tool copies, moves, and deletes save files with proper handling of the index file.

## basic usage

Assume we are in the save file directory, usually at './www/save/'.

* show info of save files
```
rpgmv-savetool ls
```


* copy save 1 to a backup file
```
rpgmv-savetool cp file1.rpgsave save_01.rpgarch
```

* copy the backup file to save 10
```
rpgmv-savetool cp save_01.rpgarch file10.rpgsave
```

* move save 1 to save 9
```
rpgmv-savetool mv file1.rpgsave file9.rpgsave
```

* remove save 10
```
rpgmv-savetool rm file10.rpgsave
```

## advanced

The RPG maker mv save files are actually handled as a whole directory. You could designate savefiles with comma-separated list of save numbers.

* copy all save files to another directory
```
rpgmv-savetool cp ./ ../save_backup/
```

* copy all save files to a single archive file
```
# use -k to keep gaps between save numbers
rpgmv-savetool cp -k ./ backup_all.rpgarch

# show contents of the backup file
rpgmv-savetool ls backup_all.rpgarch
```

* show contents of the archive fiel

* copy save 1, 3, 5 to a backup file's save slot #11, 12, 13
```
rpgmv-savetool cp ./#1,3,5 save_02.rpgarch#11
```

* copy all save slots in a backup file to save 10, 11, 12, ...
```
rpgmv-savetool cp save_02.rpgarch ./#10-
```

* move savefile 1 to 5 to 11 to 15
```
rpgmv-savetool mv -k ./#1-5 ./#11-
```

* remove all savefiles larger than 19
```
rpgmv-savetool rm ./#20-
```

