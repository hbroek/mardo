#import <Cocoa/Cocoa.h>

extern void goSelectAll(void);
extern void goCopyAll(void);
extern void goMenu(const char *id);

// Called from Go: see tabbing_darwin.go.
void disableWindowTabbing(void) {
    [NSWindow setAllowsAutomaticWindowTabbing:NO];
}

@interface SelectAllTarget : NSObject
- (void)selectAllClicked:(id)sender;
- (void)copyAllClicked:(id)sender;
- (void)appItemClicked:(NSMenuItem *)sender;
@end

@implementation SelectAllTarget
- (void)selectAllClicked:(id)sender {
    goSelectAll();
}
- (void)copyAllClicked:(id)sender {
    goCopyAll();
}
- (void)appItemClicked:(NSMenuItem *)sender {
    goMenu([(NSString *)sender.representedObject UTF8String]);
}
@end

// Edit > Select All selects the whole web view by default. Send it to the page instead, which
// knows whether the bookmarks, the page or the chat is meant.
void routeSelectAll(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        static SelectAllTarget *target;
        if (!target) target = [SelectAllTarget new];
        NSMenu *edit = [[[NSApp mainMenu] itemWithTitle:@"Edit"] submenu];
        NSInteger copyAt = -1;
        BOOL haveCopyAll = NO;
        for (NSMenuItem *item in edit.itemArray) {
            if (item.action == @selector(selectAll:)) {
                item.target = target;
                item.action = @selector(selectAllClicked:);
            }
            if (item.action == @selector(copy:)) copyAt = [edit indexOfItem:item];
            if (item.action == @selector(copyAllClicked:)) haveCopyAll = YES;
        }
        // About Mardo is the first entry of the Mardo menu, then the app's settings: Site Credentials and the AI setup.
        NSMenu *app = [[[NSApp mainMenu] itemAtIndex:0] submenu];
        if (app && [app indexOfItemWithTitle:@"AI Assistant…"] < 0) {
            NSInteger at = 0; // at the top: Wails' app menu starts with Hide
            NSArray *entries = @[@[@"About Mardo", @"about"], @[@"-"], @[@"Site Credentials…", @"credentials"], @[@"AI Assistant…", @"ai"]];
            for (NSArray *e in entries) {
                if ([e[0] isEqualToString:@"-"]) { [app insertItem:[NSMenuItem separatorItem] atIndex:at++]; continue; }
                NSMenuItem *m = [[NSMenuItem alloc] initWithTitle:e[0] action:@selector(appItemClicked:) keyEquivalent:@""];
                m.target = target;
                m.representedObject = e[1];
                [app insertItem:m atIndex:at++];
            }
            [app insertItem:[NSMenuItem separatorItem] atIndex:at];
        }
        // Copy All (the page's markdown) goes right below Copy.
        if (copyAt >= 0 && !haveCopyAll) {
            NSMenuItem *all = [[NSMenuItem alloc] initWithTitle:@"Copy All" action:@selector(copyAllClicked:) keyEquivalent:@"c"];
            all.keyEquivalentModifierMask = NSEventModifierFlagCommand | NSEventModifierFlagShift;
            all.target = target;
            [edit insertItem:all atIndex:copyAt + 1];
        }
        // Find: Cmd+F opens the page's find bar, Cmd+G and Shift+Cmd+G step through the matches.
        if (copyAt >= 0 && [edit indexOfItemWithTitle:@"Find\u2026"] < 0) {
            NSArray *finds = @[
                @[@"Find\u2026", @"find", @"f", @(NSEventModifierFlagCommand)],
                @[@"Find and Replace\u2026", @"replace", @"f", @(NSEventModifierFlagCommand | NSEventModifierFlagOption)],
                @[@"Find Next", @"find-next", @"g", @(NSEventModifierFlagCommand)],
                @[@"Find Previous", @"find-prev", @"g", @(NSEventModifierFlagCommand | NSEventModifierFlagShift)],
            ];
            NSInteger at = [edit numberOfItems]; // below Select All (the end of the standard items, if it is not found)
            for (NSMenuItem *item in edit.itemArray) {
                if (item.action == @selector(selectAllClicked:)) at = [edit indexOfItem:item] + 1;
            }
            [edit insertItem:[NSMenuItem separatorItem] atIndex:at++];
            for (NSArray *f in finds) {
                NSMenuItem *m = [[NSMenuItem alloc] initWithTitle:f[0] action:@selector(appItemClicked:) keyEquivalent:f[2]];
                m.keyEquivalentModifierMask = [f[3] unsignedIntegerValue];
                m.target = target;
                m.representedObject = f[1];
                [edit insertItem:m atIndex:at++];
            }
        }
    });
}
